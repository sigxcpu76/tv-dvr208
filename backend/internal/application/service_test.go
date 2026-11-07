package application_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/sigxcpu/tv-dvr208/backend/internal/application"
	"github.com/sigxcpu/tv-dvr208/backend/internal/domain"
)

type fakeDVRAdapter struct {
	lastPTZChannel  int
	lastPTZCommand  domain.PTZCommand
	lastMotionCfg   domain.MotionDetectionConfig
	lastOverlayCfg  domain.VideoOverlayConfig
	downloadPayload []byte
	remuxedInput    []byte
}

func (f *fakeDVRAdapter) GetDeviceInfo(ctx context.Context) (*domain.DeviceInfo, error) {
	return &domain.DeviceInfo{DeviceName: "TV-DVR208", Model: "TV-DVR208", FirmwareVersion: "V3.4.7"}, nil
}
func (f *fakeDVRAdapter) GetSystemTime(ctx context.Context) (*domain.SystemTime, error) {
	return &domain.SystemTime{TimeMode: "NTP", LocalTime: "2026-11-07T16:00:00+02:00"}, nil
}
func (f *fakeDVRAdapter) GetStorageVolumes(ctx context.Context) ([]domain.StorageVolume, error) {
	return []domain.StorageVolume{{ID: 1, Name: "hdda", CapacityMB: 953869, Status: "ok"}}, nil
}
func (f *fakeDVRAdapter) GetNetworkInterface(ctx context.Context) (*domain.NetworkInterface, error) {
	return &domain.NetworkInterface{ID: 1, IPAddress: "192.168.27.60"}, nil
}
func (f *fakeDVRAdapter) GetUsers(ctx context.Context) ([]domain.UserAccount, error) {
	return []domain.UserAccount{{ID: 2, UserName: "operator", UserLevel: "Operator"}}, nil
}
func (f *fakeDVRAdapter) RebootDevice(ctx context.Context) error { return nil }

func (f *fakeDVRAdapter) ListVideoChannels(ctx context.Context) ([]domain.VideoChannel, error) {
	return []domain.VideoChannel{
		{ID: 1, InputPort: 1, Enabled: true, Name: "Terasa", VideoFormat: "PAL", ResDesc: "1080P25"},
		{ID: 2, InputPort: 2, Enabled: true, Name: "Curte", VideoFormat: "PAL", ResDesc: "1080P25"},
	}, nil
}
func (f *fakeDVRAdapter) GetStreamConfig(ctx context.Context, trackID int) (*domain.StreamConfig, error) {
	if trackID%100 == 1 {
		return &domain.StreamConfig{TrackID: trackID, ChannelID: trackID / 100, VideoCodecType: "H.264", ResolutionWidth: 1920, ResolutionHeight: 1080}, nil
	}
	return &domain.StreamConfig{TrackID: trackID, ChannelID: trackID / 100, VideoCodecType: "H.264", ResolutionWidth: 960, ResolutionHeight: 480}, nil
}
func (f *fakeDVRAdapter) CaptureSnapshot(ctx context.Context, trackID int) ([]byte, error) {
	return []byte{0xff, 0xd8, 0xff, 0xd9}, nil
}
func (f *fakeDVRAdapter) GetVideoOverlays(ctx context.Context, channelID int) (*domain.VideoOverlayConfig, error) {
	return &domain.VideoOverlayConfig{
		ChannelID:   channelID,
		NormalizedW: 704,
		NormalizedH: 576,
		DateTime:    domain.DateTimeOverlay{Enabled: true, PositionX: 16, PositionY: 32},
		ChannelName: domain.ChannelNameOverlay{Enabled: true, PositionX: 512, PositionY: 512},
	}, nil
}
func (f *fakeDVRAdapter) UpdateVideoOverlays(ctx context.Context, channelID int, cfg domain.VideoOverlayConfig) error {
	f.lastOverlayCfg = cfg
	return nil
}
func (f *fakeDVRAdapter) GetMotionDetection(ctx context.Context, channelID int) (*domain.MotionDetectionConfig, error) {
	hex := strings.Repeat("fffffc", domain.MotionRows)
	grid, _ := domain.ParseMotionGridHex(hex)
	return &domain.MotionDetectionConfig{
		ChannelID:        channelID,
		Enabled:          true,
		SensitivityLevel: 3,
		GridMapHex:       hex,
		Grid:             grid,
	}, nil
}
func (f *fakeDVRAdapter) UpdateMotionDetection(ctx context.Context, channelID int, cfg domain.MotionDetectionConfig) error {
	f.lastMotionCfg = cfg
	return nil
}
func (f *fakeDVRAdapter) GetPrivacyMask(ctx context.Context, channelID int) (*domain.PrivacyMaskConfig, error) {
	return &domain.PrivacyMaskConfig{ChannelID: channelID, NormalizedW: 704, NormalizedH: 576}, nil
}
func (f *fakeDVRAdapter) UpdatePrivacyMask(ctx context.Context, channelID int, cfg domain.PrivacyMaskConfig) error {
	return nil
}

func (f *fakeDVRAdapter) ContinuousMove(ctx context.Context, channelID int, cmd domain.PTZCommand) error {
	f.lastPTZChannel = channelID
	f.lastPTZCommand = cmd
	return nil
}
func (f *fakeDVRAdapter) GotoPreset(ctx context.Context, channelID int, presetID int) error {
	return nil
}
func (f *fakeDVRAdapter) SetPreset(ctx context.Context, channelID int, presetID int) error {
	return nil
}
func (f *fakeDVRAdapter) Zoom3D(ctx context.Context, channelID int, pos domain.PTZPosition3D) error {
	return nil
}

func (f *fakeDVRAdapter) GetDailyDistribution(ctx context.Context, trackID int, year int, month int) ([]domain.RecordingDaySpan, error) {
	return []domain.RecordingDaySpan{{DayNumber: 7, HasRecords: true, RecordType: "timing"}}, nil
}
func (f *fakeDVRAdapter) SearchRecordings(ctx context.Context, trackID int, startTime, endTime time.Time) ([]domain.RecordingSegment, error) {
	return []domain.RecordingSegment{
		{
			TrackID:     trackID,
			ChannelID:   trackID / 100,
			StartTime:   startTime,
			EndTime:     endTime,
			PlaybackURI: "rtsp://192.168.27.60/Streaming/tracks/101/?starttime=20261107T120000Z&endtime=20261107T121310Z",
			SizeBytes:   262040376,
		},
	}, nil
}
func (f *fakeDVRAdapter) OpenRecordingDownloadStream(ctx context.Context, playbackURI string) (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(f.downloadPayload)), nil
}

func (f *fakeDVRAdapter) StreamLiveMP4(ctx context.Context, trackID int, w io.Writer) error {
	_, err := w.Write([]byte("fMP4-LIVE"))
	return err
}
func (f *fakeDVRAdapter) StreamArchiveRTSPToMP4(ctx context.Context, trackID int, startTime, endTime time.Time, w io.Writer) error {
	_, err := w.Write([]byte("fMP4-VOD"))
	return err
}
func (f *fakeDVRAdapter) RemuxMPEGPSStreamToMP4(ctx context.Context, mpegPSReader io.Reader, w io.Writer) error {
	b, err := io.ReadAll(mpegPSReader)
	if err != nil {
		return err
	}
	f.remuxedInput = b
	_, err = w.Write([]byte("MP4-REMUXED"))
	return err
}

func TestApplicationServiceUseCases(t *testing.T) {
	ctx := context.Background()
	adapter := &fakeDVRAdapter{}
	svc := application.NewService(adapter, adapter, adapter, adapter, adapter)

	// 1. System Overview
	ov, err := svc.GetSystemOverview(ctx)
	if err != nil {
		t.Fatalf("GetSystemOverview error: %v", err)
	}
	if ov.DeviceInfo.Model != "TV-DVR208" || len(ov.Storage) != 1 || ov.Network.IPAddress != "192.168.27.60" {
		t.Fatalf("unexpected overview: %+v", ov)
	}

	// 2. Channels with enriched stream configs
	chs, err := svc.ListChannelsWithStreams(ctx)
	if err != nil || len(chs) != 2 {
		t.Fatalf("ListChannelsWithStreams error: %v", err)
	}
	if chs[0].MainStream == nil || chs[0].MainStream.ResolutionWidth != 1920 || chs[0].SubStream.ResolutionWidth != 960 {
		t.Fatalf("expected enriched main & sub stream configs, got %+v", chs[0])
	}

	// 3. DownloadRecordingMP4 strips 40-byte IMKH header before remuxing
	imkh := make([]byte, domain.IMKHHeaderSize)
	copy(imkh, []byte("IMKH\x01\x01"))
	expectedMPEGPS := []byte{0x00, 0x00, 0x01, 0xba, 0x21, 0x00, 0x01}
	adapter.downloadPayload = append(imkh, expectedMPEGPS...)

	var out bytes.Buffer
	if err := svc.DownloadRecordingMP4(ctx, "rtsp://192.168.27.60/Streaming/tracks/101/?starttime=...", &out); err != nil {
		t.Fatalf("DownloadRecordingMP4 error: %v", err)
	}
	if !bytes.Equal(adapter.remuxedInput, expectedMPEGPS) {
		t.Fatalf("expected 40-byte IMKH header to be stripped; got %x", adapter.remuxedInput)
	}
	if out.String() != "MP4-REMUXED" {
		t.Fatalf("expected MP4-REMUXED output, got %s", out.String())
	}

	// 3b. StreamArchivePlayback uses fast ISAPI download + IMKH strip whenever playbackURI is present,
	// even if startTime and endTime are also supplied
	var playOut bytes.Buffer
	st := time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC)
	et := time.Date(2026, 11, 7, 12, 13, 10, 0, time.UTC)
	if err := svc.StreamArchivePlayback(ctx, 1, st, et, "rtsp://192.168.27.60/Streaming/tracks/101/?starttime=...", &playOut); err != nil {
		t.Fatalf("StreamArchivePlayback error: %v", err)
	}
	if playOut.String() != "MP4-REMUXED" {
		t.Fatalf("expected StreamArchivePlayback with playbackURI to use IMKH-stripped download remux, got %s", playOut.String())
	}

	// 4. Motion Detection update encodes boolean grid to hex
	var grid domain.MotionGrid
	grid[0][0] = true
	err = svc.UpdateChannelMotionDetection(ctx, 1, domain.MotionDetectionConfig{
		Enabled:          true,
		SensitivityLevel: 4,
		Grid:             grid,
	})
	if err != nil {
		t.Fatalf("UpdateChannelMotionDetection error: %v", err)
	}
	if !strings.HasPrefix(adapter.lastMotionCfg.GridMapHex, "800000") {
		t.Fatalf("expected GridMapHex to start with 800000, got %s", adapter.lastMotionCfg.GridMapHex)
	}

	// 5. PTZ Move and Stop
	if err := svc.PTZMove(ctx, 3, domain.PTZCommand{Pan: 40, Tilt: -20, Zoom: 0}); err != nil {
		t.Fatalf("PTZMove error: %v", err)
	}
	if adapter.lastPTZChannel != 3 || adapter.lastPTZCommand.Pan != 40 {
		t.Fatalf("unexpected PTZ state: ch=%d cmd=%+v", adapter.lastPTZChannel, adapter.lastPTZCommand)
	}
	if err := svc.PTZStop(ctx, 3); err != nil {
		t.Fatalf("PTZStop error: %v", err)
	}
	if adapter.lastPTZCommand.Pan != 0 || adapter.lastPTZCommand.Tilt != 0 {
		t.Fatalf("expected PTZ stop (0,0,0), got %+v", adapter.lastPTZCommand)
	}

	// 6. Invalid channel rejected
	if err := svc.PTZMove(ctx, 99, domain.PTZCommand{}); !errors.Is(err, domain.ErrInvalidChannel) {
		t.Fatalf("expected ErrInvalidChannel, got %v", err)
	}
}
