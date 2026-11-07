package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sigxcpu/tv-dvr208/backend/internal/domain"
	"github.com/sigxcpu/tv-dvr208/backend/internal/ports"
)

const maxConcurrentLiveStreams = 4

type activeStream struct {
	id        uint64
	channelID int
	cancel    context.CancelFunc
}

// Handler is the driving HTTP adapter that exposes the DVRUseCase port as a JSON REST & media streaming API
// and serves the embedded React SPA via io/fs.FS.
type Handler struct {
	useCase    ports.DVRUseCase
	staticFS   fs.FS
	fileServer http.Handler
	mux        *http.ServeMux

	streamMu          sync.Mutex
	nextStreamID      uint64
	liveStreams       []activeStream
	archivePlayID     uint64
	archivePlayCancel context.CancelFunc
}

// NewHandler creates a new HTTP Handler around a ports.DVRUseCase implementation and an optional embedded fs.FS.
func NewHandler(useCase ports.DVRUseCase, staticFS fs.FS) *Handler {
	h := &Handler{
		useCase:  useCase,
		staticFS: staticFS,
		mux:      http.NewServeMux(),
	}
	if staticFS != nil {
		h.fileServer = http.FileServerFS(staticFS)
	}
	h.registerRoutes()
	return h
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// CORS headers for local SPA development and cross-origin access
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	h.mux.ServeHTTP(w, r)
}

func (h *Handler) registerRoutes() {
	h.mux.HandleFunc("GET /api/health", h.handleHealth)
	h.mux.HandleFunc("GET /api/system/overview", h.handleSystemOverview)
	h.mux.HandleFunc("GET /api/system/device-info", h.handleDeviceInfo)
	h.mux.HandleFunc("POST /api/system/reboot", h.handleReboot)

	h.mux.HandleFunc("GET /api/channels", h.handleListChannels)
	h.mux.HandleFunc("GET /api/channels/{id}/snapshot", h.handleSnapshot)
	h.mux.HandleFunc("GET /api/channels/{id}/live.mp4", h.handleLiveStream)

	h.mux.HandleFunc("POST /api/channels/{id}/ptz/move", h.handlePTZMove)
	h.mux.HandleFunc("POST /api/channels/{id}/ptz/stop", h.handlePTZStop)
	h.mux.HandleFunc("POST /api/channels/{id}/ptz/presets/{presetId}", h.handlePTZPreset)
	h.mux.HandleFunc("POST /api/channels/{id}/ptz/zoom3d", h.handlePTZZoom3D)

	h.mux.HandleFunc("GET /api/channels/{id}/overlays", h.handleGetOverlays)
	h.mux.HandleFunc("PUT /api/channels/{id}/overlays", h.handleUpdateOverlays)
	h.mux.HandleFunc("GET /api/channels/{id}/motion", h.handleGetMotion)
	h.mux.HandleFunc("PUT /api/channels/{id}/motion", h.handleUpdateMotion)
	h.mux.HandleFunc("GET /api/channels/{id}/privacy-mask", h.handleGetPrivacyMask)
	h.mux.HandleFunc("PUT /api/channels/{id}/privacy-mask", h.handleUpdatePrivacyMask)

	h.mux.HandleFunc("GET /api/archive/channels/{id}/calendar", h.handleArchiveCalendar)
	h.mux.HandleFunc("GET /api/archive/channels/{id}/search", h.handleArchiveSearch)
	h.mux.HandleFunc("GET /api/archive/channels/{id}/play.mp4", h.handleArchivePlay)
	h.mux.HandleFunc("GET /api/archive/download.mp4", h.handleArchiveDownload)

	if h.staticFS != nil {
		h.mux.HandleFunc("/", h.handleSPA)
	}
}

func (h *Handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) handleSystemOverview(w http.ResponseWriter, r *http.Request) {
	ov, err := h.useCase.GetSystemOverview(r.Context())
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, ov)
}

func (h *Handler) handleDeviceInfo(w http.ResponseWriter, r *http.Request) {
	info, err := h.useCase.GetDeviceInfo(r.Context())
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, info)
}

func (h *Handler) handleReboot(w http.ResponseWriter, r *http.Request) {
	if err := h.useCase.RebootDevice(r.Context()); err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "rebooting"})
}

func (h *Handler) handleListChannels(w http.ResponseWriter, r *http.Request) {
	chs, err := h.useCase.ListChannelsWithStreams(r.Context())
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, chs)
}

func (h *Handler) handleSnapshot(w http.ResponseWriter, r *http.Request) {
	channelID, err := parseIDParam(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	streamType := parseIntQuery(r, "stream", domain.StreamTypeMain)
	jpegBytes, err := h.useCase.CaptureSnapshot(r.Context(), channelID, streamType)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	w.Header().Set("Content-Type", "image/jpeg")
	setNoCacheHeaders(w)
	if r.URL.Query().Get("download") == "1" {
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="ch%d_%s.jpg"`, channelID, time.Now().Format("20060102_150405")))
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(jpegBytes)
}

func (h *Handler) handleLiveStream(w http.ResponseWriter, r *http.Request) {
	channelID, err := parseIDParam(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	streamType := parseIntQuery(r, "stream", domain.StreamTypeSub)
	if err := domain.ValidateChannelID(channelID); err != nil {
		writeDomainError(w, err)
		return
	}
	if err := domain.ValidateStreamType(streamType); err != nil {
		writeDomainError(w, err)
		return
	}

	ctx, streamID := h.registerLiveStream(r.Context(), channelID)
	defer h.unregisterLiveStream(streamID)

	w.Header().Set("Content-Type", "video/mp4")
	setNoCacheHeaders(w)
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	_ = h.useCase.StreamLiveVideo(ctx, channelID, streamType, w)
}

func (h *Handler) handlePTZMove(w http.ResponseWriter, r *http.Request) {
	channelID, err := parseIDParam(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	var cmd domain.PTZCommand
	if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if err := h.useCase.PTZMove(r.Context(), channelID, cmd); err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) handlePTZStop(w http.ResponseWriter, r *http.Request) {
	channelID, err := parseIDParam(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.useCase.PTZStop(r.Context(), channelID); err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "stopped"})
}

func (h *Handler) handlePTZPreset(w http.ResponseWriter, r *http.Request) {
	channelID, err := parseIDParam(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	presetID, err := parseIDParam(r, "presetId")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	var body struct {
		Action string `json:"action"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if body.Action == "" {
		body.Action = "goto"
	}
	if err := h.useCase.PTZPreset(r.Context(), channelID, presetID, body.Action); err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) handlePTZZoom3D(w http.ResponseWriter, r *http.Request) {
	channelID, err := parseIDParam(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	var pos domain.PTZPosition3D
	if err := json.NewDecoder(r.Body).Decode(&pos); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if err := h.useCase.PTZZoom3D(r.Context(), channelID, pos); err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) handleGetOverlays(w http.ResponseWriter, r *http.Request) {
	channelID, err := parseIDParam(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	cfg, err := h.useCase.GetChannelOverlays(r.Context(), channelID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, cfg)
}

func (h *Handler) handleUpdateOverlays(w http.ResponseWriter, r *http.Request) {
	channelID, err := parseIDParam(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	var cfg domain.VideoOverlayConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if err := h.useCase.UpdateChannelOverlays(r.Context(), channelID, cfg); err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "saved"})
}

func (h *Handler) handleGetMotion(w http.ResponseWriter, r *http.Request) {
	channelID, err := parseIDParam(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	cfg, err := h.useCase.GetChannelMotionDetection(r.Context(), channelID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, cfg)
}

func (h *Handler) handleUpdateMotion(w http.ResponseWriter, r *http.Request) {
	channelID, err := parseIDParam(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	var cfg domain.MotionDetectionConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if err := h.useCase.UpdateChannelMotionDetection(r.Context(), channelID, cfg); err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "saved"})
}

func (h *Handler) handleGetPrivacyMask(w http.ResponseWriter, r *http.Request) {
	channelID, err := parseIDParam(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	cfg, err := h.useCase.GetChannelPrivacyMask(r.Context(), channelID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, cfg)
}

func (h *Handler) handleUpdatePrivacyMask(w http.ResponseWriter, r *http.Request) {
	channelID, err := parseIDParam(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	var cfg domain.PrivacyMaskConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if err := h.useCase.UpdateChannelPrivacyMask(r.Context(), channelID, cfg); err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "saved"})
}

func (h *Handler) handleArchiveCalendar(w http.ResponseWriter, r *http.Request) {
	h.cancelAllLiveStreams()
	channelID, err := parseIDParam(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	now := time.Now()
	year := parseIntQuery(r, "year", now.Year())
	month := parseIntQuery(r, "month", int(now.Month()))

	days, err := h.useCase.GetMonthlyRecordingCalendar(r.Context(), channelID, year, month)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, days)
}

func (h *Handler) handleArchiveSearch(w http.ResponseWriter, r *http.Request) {
	h.cancelAllLiveStreams()
	channelID, err := parseIDParam(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	startStr := r.URL.Query().Get("start")
	endStr := r.URL.Query().Get("end")
	if startStr == "" || endStr == "" {
		writeError(w, http.StatusBadRequest, "start and end RFC3339 query parameters are required")
		return
	}
	st, err := time.Parse(time.RFC3339, startStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid start time format (use RFC3339)")
		return
	}
	et, err := time.Parse(time.RFC3339, endStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid end time format (use RFC3339)")
		return
	}

	segs, err := h.useCase.SearchChannelRecordings(r.Context(), domain.RecordingSearchQuery{
		ChannelID: channelID,
		StartTime: st,
		EndTime:   et,
	})
	if err != nil {
		writeDomainError(w, err)
		return
	}
	if segs == nil {
		segs = []domain.RecordingSegment{}
	}
	writeJSON(w, http.StatusOK, segs)
}

func (h *Handler) handleArchivePlay(w http.ResponseWriter, r *http.Request) {
	h.cancelAllLiveStreams()
	channelID, err := parseIDParam(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := domain.ValidateChannelID(channelID); err != nil {
		writeDomainError(w, err)
		return
	}

	playbackURI := r.URL.Query().Get("uri")
	var st, et time.Time
	if s := r.URL.Query().Get("start"); s != "" {
		st, _ = time.Parse(time.RFC3339, s)
	}
	if e := r.URL.Query().Get("end"); e != "" {
		et, _ = time.Parse(time.RFC3339, e)
	}
	if playbackURI == "" && (st.IsZero() || et.IsZero()) {
		writeError(w, http.StatusBadRequest, "either uri or start+end query parameters are required")
		return
	}

	ctx, playID := h.registerArchivePlayStream(r.Context())
	defer h.unregisterArchivePlayStream(playID)

	w.Header().Set("Content-Type", "video/mp4")
	setNoCacheHeaders(w)
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	_ = h.useCase.StreamArchivePlayback(ctx, channelID, st, et, playbackURI, w)
}

func (h *Handler) handleArchiveDownload(w http.ResponseWriter, r *http.Request) {
	playbackURI := r.URL.Query().Get("uri")
	if strings.TrimSpace(playbackURI) == "" {
		writeError(w, http.StatusBadRequest, "uri query parameter is required")
		return
	}
	filename := r.URL.Query().Get("filename")
	if filename == "" {
		filename = fmt.Sprintf("tv-dvr208_%s.mp4", time.Now().Format("20060102_150405"))
	}
	if !strings.HasSuffix(strings.ToLower(filename), ".mp4") {
		filename += ".mp4"
	}

	w.Header().Set("Content-Type", "video/mp4")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename=%q`, filename))
	setNoCacheHeaders(w)
	w.WriteHeader(http.StatusOK)

	_ = h.useCase.DownloadRecordingMP4(r.Context(), playbackURI, w)
}

func (h *Handler) handleSPA(w http.ResponseWriter, r *http.Request) {
	relPath := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
	if relPath != "" && relPath != "." && relPath != "index.html" {
		if info, err := fs.Stat(h.staticFS, relPath); err == nil && !info.IsDir() {
			if strings.HasPrefix(relPath, "assets/") {
				// Vite content-hashed bundle assets are immutable and safe to cache long-term
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			} else {
				setNoCacheHeaders(w)
			}
			h.fileServer.ServeHTTP(w, r)
			return
		}
	}

	// Navigating or refreshing the SPA root terminates lingering streams from the prior page lifecycle
	h.cancelAllLiveStreams()

	// Serve embedded index.html for root "/" and client-side SPA routes (never cached by Cloudflare or browser)
	indexBytes, err := fs.ReadFile(h.staticFS, "index.html")
	if err != nil {
		writeError(w, http.StatusNotFound, "embedded index.html not found")
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	setNoCacheHeaders(w)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(indexBytes)
}

func (h *Handler) registerLiveStream(parent context.Context, channelID int) (context.Context, uint64) {
	ctx, cancel := context.WithCancel(parent)
	h.streamMu.Lock()
	defer h.streamMu.Unlock()

	// Cancel any existing live stream for the same channel (e.g., stream quality switch)
	kept := h.liveStreams[:0]
	for _, s := range h.liveStreams {
		if s.channelID == channelID {
			s.cancel()
		} else {
			kept = append(kept, s)
		}
	}
	h.liveStreams = kept

	// Evict oldest streams if at capacity so the browser's 6-connection HTTP/1.1 pool is never saturated
	for len(h.liveStreams) >= maxConcurrentLiveStreams {
		h.liveStreams[0].cancel()
		h.liveStreams = h.liveStreams[1:]
	}

	h.nextStreamID++
	id := h.nextStreamID
	h.liveStreams = append(h.liveStreams, activeStream{
		id:        id,
		channelID: channelID,
		cancel:    cancel,
	})
	return ctx, id
}

func (h *Handler) unregisterLiveStream(id uint64) {
	h.streamMu.Lock()
	defer h.streamMu.Unlock()
	for i, s := range h.liveStreams {
		if s.id == id {
			s.cancel()
			h.liveStreams = append(h.liveStreams[:i], h.liveStreams[i+1:]...)
			return
		}
	}
}

func (h *Handler) cancelAllLiveStreams() {
	h.streamMu.Lock()
	defer h.streamMu.Unlock()
	for _, s := range h.liveStreams {
		s.cancel()
	}
	h.liveStreams = nil
	if h.archivePlayCancel != nil {
		h.archivePlayCancel()
		h.archivePlayCancel = nil
	}
}

func (h *Handler) registerArchivePlayStream(parent context.Context) (context.Context, uint64) {
	ctx, cancel := context.WithCancel(parent)
	h.streamMu.Lock()
	defer h.streamMu.Unlock()
	if h.archivePlayCancel != nil {
		h.archivePlayCancel()
	}
	h.nextStreamID++
	id := h.nextStreamID
	h.archivePlayID = id
	h.archivePlayCancel = cancel
	return ctx, id
}

func (h *Handler) unregisterArchivePlayStream(id uint64) {
	h.streamMu.Lock()
	defer h.streamMu.Unlock()
	if h.archivePlayID == id && h.archivePlayCancel != nil {
		h.archivePlayCancel()
		h.archivePlayCancel = nil
	}
}

func parseIDParam(r *http.Request, name string) (int, error) {
	valStr := r.PathValue(name)
	id, err := strconv.Atoi(valStr)
	if err != nil {
		return 0, fmt.Errorf("invalid %s parameter %q", name, valStr)
	}
	return id, nil
}

func parseIntQuery(r *http.Request, key string, defaultVal int) int {
	valStr := r.URL.Query().Get(key)
	if valStr == "" {
		return defaultVal
	}
	if v, err := strconv.Atoi(valStr); err == nil {
		return v
	}
	return defaultVal
}

func setNoCacheHeaders(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
	w.Header().Set("CDN-Cache-Control", "no-store")
	w.Header().Set("Cloudflare-CDN-Cache-Control", "no-store")
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	setNoCacheHeaders(w)
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func writeDomainError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrInvalidChannel),
		errors.Is(err, domain.ErrInvalidStreamType),
		errors.Is(err, domain.ErrInvalidTimeRange),
		errors.Is(err, domain.ErrInvalidMotionGrid),
		errors.Is(err, domain.ErrInvalidPTZSpeed),
		errors.Is(err, domain.ErrInvalidOverlayCoord):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, domain.ErrAuthentication):
		writeError(w, http.StatusUnauthorized, err.Error())
	default:
		writeError(w, http.StatusBadGateway, err.Error())
	}
}
