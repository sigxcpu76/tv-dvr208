package ports

import (
	"context"
	"io"
	"time"

	"github.com/sigxcpu/tv-dvr208/backend/internal/domain"
)

// SystemOverview bundles device info, time, storage, network, and users for the dashboard & system settings.
type SystemOverview struct {
	DeviceInfo *domain.DeviceInfo        `json:"deviceInfo"`
	SystemTime *domain.SystemTime        `json:"systemTime"`
	Storage    []domain.StorageVolume    `json:"storage"`
	Network    *domain.NetworkInterface  `json:"network"`
	Users      []domain.UserAccount      `json:"users"`
}

// DVRUseCase defines the inbound application port consumed by HTTP/WebSocket driving adapters.
type DVRUseCase interface {
	// System
	GetSystemOverview(ctx context.Context) (*SystemOverview, error)
	GetDeviceInfo(ctx context.Context) (*domain.DeviceInfo, error)
	RebootDevice(ctx context.Context) error

	// Channels & Live Streaming
	ListChannelsWithStreams(ctx context.Context) ([]domain.VideoChannel, error)
	CaptureSnapshot(ctx context.Context, channelID int, streamType int) ([]byte, error)
	StreamLiveVideo(ctx context.Context, channelID int, streamType int, w io.Writer) error

	// PTZ
	PTZMove(ctx context.Context, channelID int, cmd domain.PTZCommand) error
	PTZStop(ctx context.Context, channelID int) error
	PTZPreset(ctx context.Context, channelID int, presetID int, action string) error
	PTZZoom3D(ctx context.Context, channelID int, pos domain.PTZPosition3D) error

	// Archive Calendar, Search, Playback & Download
	GetMonthlyRecordingCalendar(ctx context.Context, channelID int, year int, month int) ([]domain.RecordingDaySpan, error)
	SearchChannelRecordings(ctx context.Context, query domain.RecordingSearchQuery) ([]domain.RecordingSegment, error)
	StreamArchivePlayback(ctx context.Context, channelID int, startTime, endTime time.Time, playbackURI string, w io.Writer) error
	DownloadRecordingMP4(ctx context.Context, playbackURI string, w io.Writer) error

	// Visual Editors (OSD, Motion Grid, Privacy Mask)
	GetChannelOverlays(ctx context.Context, channelID int) (*domain.VideoOverlayConfig, error)
	UpdateChannelOverlays(ctx context.Context, channelID int, cfg domain.VideoOverlayConfig) error
	GetChannelMotionDetection(ctx context.Context, channelID int) (*domain.MotionDetectionConfig, error)
	UpdateChannelMotionDetection(ctx context.Context, channelID int, cfg domain.MotionDetectionConfig) error
	GetChannelPrivacyMask(ctx context.Context, channelID int) (*domain.PrivacyMaskConfig, error)
	UpdateChannelPrivacyMask(ctx context.Context, channelID int, cfg domain.PrivacyMaskConfig) error
}
