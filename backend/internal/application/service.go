package application

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/sigxcpu/tv-dvr208/backend/internal/domain"
	"github.com/sigxcpu/tv-dvr208/backend/internal/ports"
)

// Service implements ports.DVRUseCase using hexagonal outbound ports.
type Service struct {
	systemPort  ports.DVRSystemPort
	channelPort ports.DVRChannelPort
	ptzPort     ports.DVRPTZPort
	archivePort ports.DVRArchivePort
	mediaPort   ports.MediaStreamPort
}

// NewService constructs a new application Service with injected outbound ports.
func NewService(
	systemPort ports.DVRSystemPort,
	channelPort ports.DVRChannelPort,
	ptzPort ports.DVRPTZPort,
	archivePort ports.DVRArchivePort,
	mediaPort ports.MediaStreamPort,
) *Service {
	return &Service{
		systemPort:  systemPort,
		channelPort: channelPort,
		ptzPort:     ptzPort,
		archivePort: archivePort,
		mediaPort:   mediaPort,
	}
}

func (s *Service) GetDeviceInfo(ctx context.Context) (*domain.DeviceInfo, error) {
	return s.systemPort.GetDeviceInfo(ctx)
}

func (s *Service) GetSystemOverview(ctx context.Context) (*ports.SystemOverview, error) {
	info, err := s.systemPort.GetDeviceInfo(ctx)
	if err != nil {
		return nil, err
	}
	sysTime, err := s.systemPort.GetSystemTime(ctx)
	if err != nil {
		return nil, err
	}
	storage, err := s.systemPort.GetStorageVolumes(ctx)
	if err != nil {
		return nil, err
	}
	network, err := s.systemPort.GetNetworkInterface(ctx)
	if err != nil {
		return nil, err
	}
	users, err := s.systemPort.GetUsers(ctx)
	if err != nil {
		return nil, err
	}
	return &ports.SystemOverview{
		DeviceInfo: info,
		SystemTime: sysTime,
		Storage:    storage,
		Network:    network,
		Users:      users,
	}, nil
}

func (s *Service) RebootDevice(ctx context.Context) error {
	return s.systemPort.RebootDevice(ctx)
}

func (s *Service) ListChannelsWithStreams(ctx context.Context) ([]domain.VideoChannel, error) {
	channels, err := s.channelPort.ListVideoChannels(ctx)
	if err != nil {
		return nil, err
	}
	for i := range channels {
		chID := channels[i].ID
		if mainTrack, err := domain.TrackID(chID, domain.StreamTypeMain); err == nil {
			if cfg, err := s.channelPort.GetStreamConfig(ctx, mainTrack); err == nil {
				channels[i].MainStream = cfg
			}
		}
		if subTrack, err := domain.TrackID(chID, domain.StreamTypeSub); err == nil {
			if cfg, err := s.channelPort.GetStreamConfig(ctx, subTrack); err == nil {
				channels[i].SubStream = cfg
			}
		}
	}
	return channels, nil
}

func (s *Service) CaptureSnapshot(ctx context.Context, channelID int, streamType int) ([]byte, error) {
	if streamType == 0 {
		streamType = domain.StreamTypeMain
	}
	trackID, err := domain.TrackID(channelID, streamType)
	if err != nil {
		return nil, err
	}
	return s.channelPort.CaptureSnapshot(ctx, trackID)
}

func (s *Service) StreamLiveVideo(ctx context.Context, channelID int, streamType int, w io.Writer) error {
	if streamType == 0 {
		streamType = domain.StreamTypeSub
	}
	trackID, err := domain.TrackID(channelID, streamType)
	if err != nil {
		return err
	}
	return s.mediaPort.StreamLiveMP4(ctx, trackID, w)
}

func (s *Service) PTZMove(ctx context.Context, channelID int, cmd domain.PTZCommand) error {
	if err := domain.ValidateChannelID(channelID); err != nil {
		return err
	}
	if err := cmd.Validate(); err != nil {
		return err
	}
	return s.ptzPort.ContinuousMove(ctx, channelID, cmd)
}

func (s *Service) PTZStop(ctx context.Context, channelID int) error {
	if err := domain.ValidateChannelID(channelID); err != nil {
		return err
	}
	return s.ptzPort.ContinuousMove(ctx, channelID, domain.PTZCommand{Pan: 0, Tilt: 0, Zoom: 0})
}

func (s *Service) PTZPreset(ctx context.Context, channelID int, presetID int, action string) error {
	if err := domain.ValidateChannelID(channelID); err != nil {
		return err
	}
	if presetID < 1 || presetID > 255 {
		return errors.New("preset ID must be between 1 and 255")
	}
	switch strings.ToLower(action) {
	case "goto", "":
		return s.ptzPort.GotoPreset(ctx, channelID, presetID)
	case "set":
		return s.ptzPort.SetPreset(ctx, channelID, presetID)
	default:
		return fmt.Errorf("unsupported preset action %q", action)
	}
}

func (s *Service) PTZZoom3D(ctx context.Context, channelID int, pos domain.PTZPosition3D) error {
	if err := domain.ValidateChannelID(channelID); err != nil {
		return err
	}
	return s.ptzPort.Zoom3D(ctx, channelID, pos)
}

func (s *Service) GetMonthlyRecordingCalendar(ctx context.Context, channelID int, year int, month int) ([]domain.RecordingDaySpan, error) {
	trackID, err := domain.TrackID(channelID, domain.StreamTypeMain)
	if err != nil {
		return nil, err
	}
	if year < 2000 || year > 2100 || month < 1 || month > 12 {
		return nil, errors.New("invalid year or month")
	}
	return s.archivePort.GetDailyDistribution(ctx, trackID, year, month)
}

func (s *Service) SearchChannelRecordings(ctx context.Context, query domain.RecordingSearchQuery) ([]domain.RecordingSegment, error) {
	if err := query.Validate(); err != nil {
		return nil, err
	}
	trackID, err := domain.TrackID(query.ChannelID, domain.StreamTypeMain)
	if err != nil {
		return nil, err
	}
	return s.archivePort.SearchRecordings(ctx, trackID, query.StartTime, query.EndTime)
}

func (s *Service) StreamArchivePlayback(ctx context.Context, channelID int, startTime, endTime time.Time, playbackURI string, w io.Writer) error {
	// Whenever an exact segment playbackURI is provided, stream via ISAPI ContentMgmt/download + 40-byte IMKH strip
	// (much faster startup and avoids TV-DVR208 RTSP VOD session stalls).
	if strings.TrimSpace(playbackURI) != "" {
		return s.DownloadRecordingMP4(ctx, playbackURI, w)
	}
	trackID, err := domain.TrackID(channelID, domain.StreamTypeMain)
	if err != nil {
		return err
	}
	if !startTime.Before(endTime) {
		return domain.ErrInvalidTimeRange
	}
	return s.mediaPort.StreamArchiveRTSPToMP4(ctx, trackID, startTime, endTime, w)
}

func (s *Service) DownloadRecordingMP4(ctx context.Context, playbackURI string, w io.Writer) error {
	if strings.TrimSpace(playbackURI) == "" {
		return errors.New("playbackURI is required")
	}
	rawStream, err := s.archivePort.OpenRecordingDownloadStream(ctx, playbackURI)
	if err != nil {
		return err
	}
	defer rawStream.Close()

	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			_ = rawStream.Close()
		case <-done:
		}
	}()

	// Strip the 40-byte Hikvision IMKH system header uncovered via Ghidra (FUN_1000f4a0)
	mpegPSReader, _, err := domain.StripIMKHHeader(rawStream)
	if err != nil {
		return err
	}
	return s.mediaPort.RemuxMPEGPSStreamToMP4(ctx, mpegPSReader, w)
}

func (s *Service) GetChannelOverlays(ctx context.Context, channelID int) (*domain.VideoOverlayConfig, error) {
	if err := domain.ValidateChannelID(channelID); err != nil {
		return nil, err
	}
	return s.channelPort.GetVideoOverlays(ctx, channelID)
}

func (s *Service) UpdateChannelOverlays(ctx context.Context, channelID int, cfg domain.VideoOverlayConfig) error {
	if err := domain.ValidateChannelID(channelID); err != nil {
		return err
	}
	if err := cfg.Validate(); err != nil {
		return err
	}
	cfg.ChannelID = channelID
	return s.channelPort.UpdateVideoOverlays(ctx, channelID, cfg)
}

func (s *Service) GetChannelMotionDetection(ctx context.Context, channelID int) (*domain.MotionDetectionConfig, error) {
	if err := domain.ValidateChannelID(channelID); err != nil {
		return nil, err
	}
	return s.channelPort.GetMotionDetection(ctx, channelID)
}

func (s *Service) UpdateChannelMotionDetection(ctx context.Context, channelID int, cfg domain.MotionDetectionConfig) error {
	if err := domain.ValidateChannelID(channelID); err != nil {
		return err
	}
	cfg.ChannelID = channelID
	if cfg.GridMapHex != "" {
		grid, err := domain.ParseMotionGridHex(cfg.GridMapHex)
		if err != nil {
			return err
		}
		cfg.Grid = grid
		cfg.GridMapHex = grid.ToHex()
	} else {
		cfg.GridMapHex = cfg.Grid.ToHex()
	}
	return s.channelPort.UpdateMotionDetection(ctx, channelID, cfg)
}

func (s *Service) GetChannelPrivacyMask(ctx context.Context, channelID int) (*domain.PrivacyMaskConfig, error) {
	if err := domain.ValidateChannelID(channelID); err != nil {
		return nil, err
	}
	return s.channelPort.GetPrivacyMask(ctx, channelID)
}

func (s *Service) UpdateChannelPrivacyMask(ctx context.Context, channelID int, cfg domain.PrivacyMaskConfig) error {
	if err := domain.ValidateChannelID(channelID); err != nil {
		return err
	}
	cfg.ChannelID = channelID
	return s.channelPort.UpdatePrivacyMask(ctx, channelID, cfg)
}
