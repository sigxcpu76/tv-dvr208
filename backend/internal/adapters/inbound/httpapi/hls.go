package httpapi

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sigxcpu/tv-dvr208/backend/internal/domain"
)

type hlsSession struct {
	key        string
	dir        string
	cancel     context.CancelFunc
	mu         sync.Mutex
	lastAccess time.Time
	done       chan struct{}
}

func (s *hlsSession) touch() {
	s.mu.Lock()
	s.lastAccess = time.Now()
	s.mu.Unlock()
}

func (s *hlsSession) expired(ttl time.Duration) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return time.Since(s.lastAccess) > ttl
}

func (h *Handler) handleLiveHLS(w http.ResponseWriter, r *http.Request) {
	channelID, err := parseIDParam(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	streamType, err := parseIDParam(r, "stream")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := domain.ValidateChannelID(channelID); err != nil {
		writeDomainError(w, err)
		return
	}
	if err := domain.ValidateStreamType(streamType); err != nil {
		writeDomainError(w, err)
		return
	}

	file := path.Base(r.PathValue("file"))
	if !isValidHLSFile(file) {
		writeError(w, http.StatusBadRequest, "invalid HLS resource")
		return
	}

	sess, err := h.getOrStartLiveHLSSession(channelID, streamType)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	sess.touch()

	targetPath := filepath.Join(sess.dir, file)
	if strings.HasSuffix(file, ".m3u8") {
		if err := waitForFile(r.Context(), targetPath, 8*time.Second); err != nil {
			writeError(w, http.StatusGatewayTimeout, "timed out waiting for HLS playlist")
			return
		}
		data, err := os.ReadFile(targetPath)
		if err != nil {
			writeError(w, http.StatusNotFound, "playlist not ready")
			return
		}
		w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
		w.Header().Set("Content-Length", strconv.Itoa(len(data)))
		setNoCacheHeaders(w)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
		return
	}

	// Serve .ts segment
	if err := waitForFile(r.Context(), targetPath, 3*time.Second); err != nil {
		writeError(w, http.StatusNotFound, "HLS segment not found")
		return
	}
	data, err := os.ReadFile(targetPath)
	if err != nil {
		writeError(w, http.StatusNotFound, "HLS segment not found")
		return
	}
	w.Header().Set("Content-Type", "video/mp2t")
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	setNoCacheHeaders(w)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (h *Handler) handleArchiveHLS(w http.ResponseWriter, r *http.Request) {
	channelID, err := parseIDParam(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := domain.ValidateChannelID(channelID); err != nil {
		writeDomainError(w, err)
		return
	}

	file := path.Base(r.PathValue("file"))
	if !isValidHLSFile(file) {
		writeError(w, http.StatusBadRequest, "invalid HLS resource")
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

	sess, err := h.getOrStartArchiveHLSSession(channelID, st, et, playbackURI)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	sess.touch()

	targetPath := filepath.Join(sess.dir, file)
	if strings.HasSuffix(file, ".m3u8") {
		if err := waitForFile(r.Context(), targetPath, 8*time.Second); err != nil {
			writeError(w, http.StatusGatewayTimeout, "timed out waiting for archive HLS playlist")
			return
		}
		data, err := os.ReadFile(targetPath)
		if err != nil {
			writeError(w, http.StatusNotFound, "archive playlist not ready")
			return
		}
		w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
		w.Header().Set("Content-Length", strconv.Itoa(len(data)))
		setNoCacheHeaders(w)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
		return
	}

	if err := waitForFile(r.Context(), targetPath, 4*time.Second); err != nil {
		writeError(w, http.StatusNotFound, "archive HLS segment not found")
		return
	}
	data, err := os.ReadFile(targetPath)
	if err != nil {
		writeError(w, http.StatusNotFound, "archive HLS segment not found")
		return
	}
	w.Header().Set("Content-Type", "video/mp2t")
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	setNoCacheHeaders(w)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (h *Handler) getOrStartLiveHLSSession(channelID, streamType int) (*hlsSession, error) {
	key := fmt.Sprintf("live-%d-%d", channelID, streamType)

	h.streamMu.Lock()
	if h.liveHLS != nil {
		if h.liveHLS.key == key {
			select {
			case <-h.liveHLS.done:
				// Previous process exited; recreate below
			default:
				sess := h.liveHLS
				h.streamMu.Unlock()
				return sess, nil
			}
		}
		h.liveHLS.cancel()
		h.liveHLS = nil
	}

	if h.archiveHLS != nil {
		h.archiveHLS.cancel()
		h.archiveHLS = nil
	}

	dir, err := os.MkdirTemp("", "tv-dvr208-live-hls-*")
	if err != nil {
		h.streamMu.Unlock()
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())
	sess := &hlsSession{
		key:        key,
		dir:        dir,
		cancel:     cancel,
		lastAccess: time.Now(),
		done:       make(chan struct{}),
	}
	h.liveHLS = sess
	h.streamMu.Unlock()

	go h.runHLSPipeline(ctx, sess, 12*time.Second, true, func(w io.Writer) error {
		return h.useCase.StreamLiveVideo(ctx, channelID, streamType, w)
	})

	return sess, nil
}

func (h *Handler) getOrStartArchiveHLSSession(channelID int, st, et time.Time, playbackURI string) (*hlsSession, error) {
	h.streamMu.Lock()
	// When requesting a .ts segment, playbackURI may not be in the query string; reuse the current session
	if playbackURI == "" && st.IsZero() && h.archiveHLS != nil {
		sess := h.archiveHLS
		h.streamMu.Unlock()
		return sess, nil
	}
	if playbackURI == "" && (st.IsZero() || et.IsZero()) {
		h.streamMu.Unlock()
		return nil, fmt.Errorf("uri or start+end query parameters are required")
	}

	key := fmt.Sprintf("archive-%d-%s-%s", channelID, playbackURI, st.Format(time.RFC3339))
	if h.archiveHLS != nil {
		if h.archiveHLS.key == key {
			select {
			case <-h.archiveHLS.done:
			default:
				sess := h.archiveHLS
				h.streamMu.Unlock()
				return sess, nil
			}
		}
		h.archiveHLS.cancel()
		h.archiveHLS = nil
	}

	if h.liveHLS != nil {
		h.liveHLS.cancel()
		h.liveHLS = nil
	}
	for _, s := range h.liveStreams {
		s.cancel()
	}
	h.liveStreams = nil

	dir, err := os.MkdirTemp("", "tv-dvr208-archive-hls-*")
	if err != nil {
		h.streamMu.Unlock()
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())
	sess := &hlsSession{
		key:        key,
		dir:        dir,
		cancel:     cancel,
		lastAccess: time.Now(),
		done:       make(chan struct{}),
	}
	h.archiveHLS = sess
	h.streamMu.Unlock()

	go h.runHLSPipeline(ctx, sess, 25*time.Second, false, func(w io.Writer) error {
		return h.useCase.StreamArchivePlayback(ctx, channelID, st, et, playbackURI, w)
	})

	return sess, nil
}

func (h *Handler) runHLSPipeline(
	ctx context.Context,
	sess *hlsSession,
	idleTTL time.Duration,
	isLive bool,
	sourceFn func(w io.Writer) error,
) {
	defer func() {
		sess.cancel()
		_ = os.RemoveAll(sess.dir)
		close(sess.done)
	}()

	// Idle reaper goroutine stops the HLS session if the client navigates away or locks the phone
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if sess.expired(idleTTL) {
					sess.cancel()
					return
				}
			}
		}
	}()

	pr, pw := io.Pipe()
	go func() {
		_ = pw.CloseWithError(sourceFn(pw))
	}()

	m3u8Path := filepath.Join(sess.dir, "index.m3u8")
	segPattern := filepath.Join(sess.dir, "seg_%05d.ts")

	var args []string
	if isLive {
		args = []string{
			"-hide_banner",
			"-loglevel", "error",
			"-f", "mp4",
			"-i", "pipe:0",
			"-c:v", "copy",
			"-an",
			"-f", "hls",
			"-hls_time", "1",
			"-hls_list_size", "6",
			"-hls_flags", "delete_segments+independent_segments+temp_file",
			"-hls_segment_filename", segPattern,
			m3u8Path,
		}
	} else {
		args = []string{
			"-hide_banner",
			"-loglevel", "error",
			"-f", "mp4",
			"-i", "pipe:0",
			"-c:v", "copy",
			"-an",
			"-f", "hls",
			"-hls_time", "2",
			"-hls_playlist_type", "event",
			"-hls_flags", "independent_segments+temp_file",
			"-hls_segment_filename", segPattern,
			m3u8Path,
		}
	}

	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	cmd.WaitDelay = 2 * time.Second
	cmd.Stdin = pr
	_ = cmd.Run()
	_ = pr.Close()
}

func isValidHLSFile(name string) bool {
	if name == "" || strings.Contains(name, "/") || strings.Contains(name, "..") {
		return false
	}
	return name == "index.m3u8" || (strings.HasPrefix(name, "seg_") && strings.HasSuffix(name, ".ts"))
}

func waitForFile(ctx context.Context, path string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		if info, err := os.Stat(path); err == nil && info.Size() > 0 {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timeout waiting for %s", path)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(50 * time.Millisecond):
		}
	}
}
