package ports

import (
	"context"
	"io"
	"time"

	"github.com/sigxcpu/tv-dvr208/backend/internal/domain"
)

// DVRSystemPort defines the driven port for system, time, storage, network, and user operations.
type DVRSystemPort interface {
	GetDeviceInfo(ctx context.Context) (*domain.DeviceInfo, error)
	GetSystemTime(ctx context.Context) (*domain.SystemTime, error)
	GetStorageVolumes(ctx context.Context) ([]domain.StorageVolume, error)
	GetNetworkInterface(ctx context.Context) (*domain.NetworkInterface, error)
	GetUsers(ctx context.Context) ([]domain.UserAccount, error)
	RebootDevice(ctx context.Context) error
}

// DVRChannelPort defines the driven port for camera channels, streams, snapshots, OSD, motion, and privacy masks.
type DVRChannelPort interface {
	ListVideoChannels(ctx context.Context) ([]domain.VideoChannel, error)
	GetStreamConfig(ctx context.Context, trackID int) (*domain.StreamConfig, error)
	CaptureSnapshot(ctx context.Context, trackID int) ([]byte, error)
	GetVideoOverlays(ctx context.Context, channelID int) (*domain.VideoOverlayConfig, error)
	UpdateVideoOverlays(ctx context.Context, channelID int, cfg domain.VideoOverlayConfig) error
	GetMotionDetection(ctx context.Context, channelID int) (*domain.MotionDetectionConfig, error)
	UpdateMotionDetection(ctx context.Context, channelID int, cfg domain.MotionDetectionConfig) error
	GetPrivacyMask(ctx context.Context, channelID int) (*domain.PrivacyMaskConfig, error)
	UpdatePrivacyMask(ctx context.Context, channelID int, cfg domain.PrivacyMaskConfig) error
}

// DVRPTZPort defines the driven port for PTZ camera control and 3D zoom.
type DVRPTZPort interface {
	ContinuousMove(ctx context.Context, channelID int, cmd domain.PTZCommand) error
	GotoPreset(ctx context.Context, channelID int, presetID int) error
	SetPreset(ctx context.Context, channelID int, presetID int) error
	Zoom3D(ctx context.Context, channelID int, pos domain.PTZPosition3D) error
}

// DVRArchivePort defines the driven port for recording calendar distribution, search, and raw download streams.
type DVRArchivePort interface {
	GetDailyDistribution(ctx context.Context, trackID int, year int, month int) ([]domain.RecordingDaySpan, error)
	SearchRecordings(ctx context.Context, trackID int, startTime, endTime time.Time) ([]domain.RecordingSegment, error)
	OpenRecordingDownloadStream(ctx context.Context, playbackURI string) (io.ReadCloser, error)
}

// MediaStreamPort defines the driven port for zero-transcode H.264 -> fragmented MP4 streaming.
type MediaStreamPort interface {
	StreamLiveMP4(ctx context.Context, trackID int, w io.Writer) error
	StreamArchiveRTSPToMP4(ctx context.Context, trackID int, startTime, endTime time.Time, w io.Writer) error
	RemuxMPEGPSStreamToMP4(ctx context.Context, mpegPSReader io.Reader, w io.Writer) error
}
