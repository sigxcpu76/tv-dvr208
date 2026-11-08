package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/sigxcpu/tv-dvr208/backend/internal/adapters/inbound/httpapi"
	"github.com/sigxcpu/tv-dvr208/backend/internal/domain"
	"github.com/sigxcpu/tv-dvr208/backend/internal/ports"
)

type stubUseCase struct {
	lastPTZCmd domain.PTZCommand
}

func (s *stubUseCase) GetSystemOverview(ctx context.Context) (*ports.SystemOverview, error) {
	return &ports.SystemOverview{
		DeviceInfo: &domain.DeviceInfo{DeviceName: "TV-DVR208", Model: "TV-DVR208"},
		SystemTime: &domain.SystemTime{TimeMode: "NTP", LocalTime: "2026-11-07T16:15:00+02:00"},
	}, nil
}
func (s *stubUseCase) GetDeviceInfo(ctx context.Context) (*domain.DeviceInfo, error) {
	return &domain.DeviceInfo{DeviceName: "TV-DVR208", Model: "TV-DVR208"}, nil
}
func (s *stubUseCase) RebootDevice(ctx context.Context) error { return nil }
func (s *stubUseCase) ListChannelsWithStreams(ctx context.Context) ([]domain.VideoChannel, error) {
	return []domain.VideoChannel{{ID: 1, Name: "Terasa", Enabled: true}}, nil
}
func (s *stubUseCase) CaptureSnapshot(ctx context.Context, channelID int, streamType int) ([]byte, error) {
	if err := domain.ValidateChannelID(channelID); err != nil {
		return nil, err
	}
	return []byte{0xff, 0xd8, 0xff, 0xd9}, nil
}
func (s *stubUseCase) StreamLiveVideo(ctx context.Context, channelID int, streamType int, w io.Writer) error {
	_, err := w.Write([]byte("LIVE-MP4-BYTES"))
	return err
}
func (s *stubUseCase) PTZMove(ctx context.Context, channelID int, cmd domain.PTZCommand) error {
	if err := domain.ValidateChannelID(channelID); err != nil {
		return err
	}
	if err := cmd.Validate(); err != nil {
		return err
	}
	s.lastPTZCmd = cmd
	return nil
}
func (s *stubUseCase) PTZStop(ctx context.Context, channelID int) error {
	s.lastPTZCmd = domain.PTZCommand{}
	return nil
}
func (s *stubUseCase) PTZPreset(ctx context.Context, channelID int, presetID int, action string) error {
	return nil
}
func (s *stubUseCase) PTZZoom3D(ctx context.Context, channelID int, pos domain.PTZPosition3D) error {
	return nil
}
func (s *stubUseCase) GetMonthlyRecordingCalendar(ctx context.Context, channelID int, year int, month int) ([]domain.RecordingDaySpan, error) {
	return []domain.RecordingDaySpan{{DayNumber: 7, HasRecords: true, RecordType: "time"}}, nil
}
func (s *stubUseCase) SearchChannelRecordings(ctx context.Context, query domain.RecordingSearchQuery) ([]domain.RecordingSegment, error) {
	if err := query.Validate(); err != nil {
		return nil, err
	}
	return []domain.RecordingSegment{{TrackID: 101, ChannelID: query.ChannelID, FileName: "seg1"}}, nil
}
func (s *stubUseCase) StreamArchivePlayback(ctx context.Context, channelID int, startTime, endTime time.Time, playbackURI string, w io.Writer) error {
	_, err := w.Write([]byte("ARCHIVE-PLAY-MP4"))
	return err
}
func (s *stubUseCase) DownloadRecordingMP4(ctx context.Context, playbackURI string, w io.Writer) error {
	_, err := w.Write([]byte("ARCHIVE-DOWNLOAD-MP4"))
	return err
}
func (s *stubUseCase) GetChannelOverlays(ctx context.Context, channelID int) (*domain.VideoOverlayConfig, error) {
	return &domain.VideoOverlayConfig{ChannelID: channelID, NormalizedW: 704, NormalizedH: 576}, nil
}
func (s *stubUseCase) UpdateChannelOverlays(ctx context.Context, channelID int, cfg domain.VideoOverlayConfig) error {
	return cfg.Validate()
}
func (s *stubUseCase) GetChannelMotionDetection(ctx context.Context, channelID int) (*domain.MotionDetectionConfig, error) {
	return &domain.MotionDetectionConfig{ChannelID: channelID, Enabled: true, SensitivityLevel: 2}, nil
}
func (s *stubUseCase) UpdateChannelMotionDetection(ctx context.Context, channelID int, cfg domain.MotionDetectionConfig) error {
	return nil
}
func (s *stubUseCase) GetChannelPrivacyMask(ctx context.Context, channelID int) (*domain.PrivacyMaskConfig, error) {
	return &domain.PrivacyMaskConfig{ChannelID: channelID, Enabled: false}, nil
}
func (s *stubUseCase) UpdateChannelPrivacyMask(ctx context.Context, channelID int, cfg domain.PrivacyMaskConfig) error {
	return nil
}

func TestHTTPAPIHandlerRoutes(t *testing.T) {
	uc := &stubUseCase{}
	fakeFS := fstest.MapFS{
		"index.html":        &fstest.MapFile{Data: []byte(`<!doctype html><html><body><div id="root"></div></body></html>`)},
		"assets/app.js":     &fstest.MapFile{Data: []byte(`console.log("tv-dvr208");`)},
	}
	h := httpapi.NewHandler(uc, fakeFS)
	srv := httptest.NewServer(h)
	defer srv.Close()

	// 0. Embedded SPA root, static asset, and SPA fallback route
	resp, err := http.Get(srv.URL + "/")
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("root SPA index failed: %v", err)
	}
	if cfHdr := resp.Header.Get("Cloudflare-CDN-Cache-Control"); cfHdr != "no-store" {
		t.Fatalf("expected Cloudflare-CDN-Cache-Control: no-store on index.html, got %q", cfHdr)
	}
	indexData, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if !strings.Contains(string(indexData), `<div id="root"></div>`) {
		t.Fatalf("unexpected root index body: %s", string(indexData))
	}

	resp, err = http.Get(srv.URL + "/assets/app.js")
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("embedded asset failed: %v", err)
	}
	if cc := resp.Header.Get("Cache-Control"); !strings.Contains(cc, "immutable") {
		t.Fatalf("expected immutable Cache-Control on hashed /assets/*, got %q", cc)
	}
	jsData, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if !strings.Contains(string(jsData), "tv-dvr208") {
		t.Fatalf("unexpected js asset body: %s", string(jsData))
	}

	resp, err = http.Get(srv.URL + "/archive")
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("SPA fallback route failed: %v", err)
	}
	fallbackData, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if !strings.Contains(string(fallbackData), `<div id="root"></div>`) {
		t.Fatalf("expected SPA fallback to serve index.html, got: %s", string(fallbackData))
	}

	// Missing static asset (/favicon.ico) returns 204 No Content without falling through to index.html
	resp, err = http.Get(srv.URL + "/favicon.ico")
	if err != nil || resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204 No Content for /favicon.ico, got %v (err=%v)", resp.StatusCode, err)
	}
	resp.Body.Close()

	// 1. Health check
	resp, err = http.Get(srv.URL + "/api/health")
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("health check failed: %v", err)
	}
	resp.Body.Close()

	// 2. Channels list
	resp, err = http.Get(srv.URL + "/api/channels")
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("channels failed: %v", err)
	}
	var chs []domain.VideoChannel
	_ = json.NewDecoder(resp.Body).Decode(&chs)
	resp.Body.Close()
	if len(chs) != 1 || chs[0].Name != "Terasa" {
		t.Fatalf("unexpected channels response: %+v", chs)
	}

	// 3. Snapshot JPEG
	resp, err = http.Get(srv.URL + "/api/channels/1/snapshot?download=1")
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("snapshot failed: %v", err)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "image/jpeg" {
		t.Fatalf("expected image/jpeg, got %s", ct)
	}
	if cd := resp.Header.Get("Content-Disposition"); !strings.Contains(cd, "attachment") {
		t.Fatalf("expected attachment Content-Disposition, got %s", cd)
	}
	resp.Body.Close()

	// 4. Live stream fMP4 & WebSocket fMP4
	resp, err = http.Get(srv.URL + "/api/channels/1/live.mp4?stream=2")
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("live.mp4 failed: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if string(body) != "LIVE-MP4-BYTES" {
		t.Fatalf("unexpected live stream body: %s", string(body))
	}

	wsReq, _ := http.NewRequest("GET", srv.URL+"/api/channels/1/live.ws?stream=2", nil)
	wsReq.Header.Set("Connection", "Upgrade")
	wsReq.Header.Set("Upgrade", "websocket")
	wsReq.Header.Set("Sec-WebSocket-Version", "13")
	wsReq.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")
	wsResp, err := http.DefaultClient.Do(wsReq)
	if err != nil || wsResp.StatusCode != http.StatusSwitchingProtocols {
		t.Fatalf("live.ws upgrade failed: status=%v err=%v", wsResp.StatusCode, err)
	}
	wsFrame, _ := io.ReadAll(wsResp.Body)
	wsResp.Body.Close()
	if len(wsFrame) < 2+len("LIVE-MP4-BYTES") || wsFrame[0] != 0x82 || string(wsFrame[2:2+len("LIVE-MP4-BYTES")]) != "LIVE-MP4-BYTES" {
		t.Fatalf("unexpected live.ws binary frame: %x", wsFrame)
	}

	// 5. PTZ Move & validation error
	ptzBody := bytes.NewBufferString(`{"pan":50,"tilt":-30,"zoom":10}`)
	resp, err = http.Post(srv.URL+"/api/channels/1/ptz/move", "application/json", ptzBody)
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("ptz move failed: %v", err)
	}
	resp.Body.Close()
	if uc.lastPTZCmd.Pan != 50 || uc.lastPTZCmd.Tilt != -30 {
		t.Fatalf("expected PTZ command recorded, got %+v", uc.lastPTZCmd)
	}

	badPtz := bytes.NewBufferString(`{"pan":500,"tilt":0,"zoom":0}`)
	resp, _ = http.Post(srv.URL+"/api/channels/1/ptz/move", "application/json", badPtz)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for out-of-range PTZ speed, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// 6. Archive search & download
	resp, err = http.Get(srv.URL + "/api/archive/channels/1/search?start=2026-11-07T00:00:00Z&end=2026-11-07T23:59:59Z")
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("archive search failed: %v", err)
	}
	resp.Body.Close()

	resp, err = http.Get(srv.URL + "/api/archive/download.mp4?uri=rtsp://192.168.27.60/Streaming/tracks/101/&filename=cam1_clip")
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("archive download failed: %v", err)
	}
	if cd := resp.Header.Get("Content-Disposition"); !strings.Contains(cd, "cam1_clip.mp4") {
		t.Fatalf("expected cam1_clip.mp4 in Content-Disposition, got %s", cd)
	}
	dlBytes, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if string(dlBytes) != "ARCHIVE-DOWNLOAD-MP4" {
		t.Fatalf("unexpected download body: %s", string(dlBytes))
	}

	// 7. HLS endpoint validation (rejects invalid resource names)
	resp, err = http.Get(srv.URL + "/api/channels/1/hls/2/invalid.txt")
	if err != nil || resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for invalid HLS file, got %v", resp.StatusCode)
	}
	resp.Body.Close()

	resp, err = http.Get(srv.URL + "/api/archive/channels/1/hls/index.m3u8")
	if err != nil || resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for archive HLS without uri/start+end, got %v", resp.StatusCode)
	}
	resp.Body.Close()
}
