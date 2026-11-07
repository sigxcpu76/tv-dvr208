package domain

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	MinChannelID = 1
	MaxChannelID = 8

	StreamTypeMain = 1
	StreamTypeSub  = 2

	NormalizedWidth  = 704
	NormalizedHeight = 576

	MotionRows = 18
	MotionCols = 22
)

// ValidateChannelID verifies that channelID is within the 8 analog HD channels of TV-DVR208.
func ValidateChannelID(channelID int) error {
	if channelID < MinChannelID || channelID > MaxChannelID {
		return ErrInvalidChannel
	}
	return nil
}

// ValidateStreamType verifies that streamType is 1 (Main 1080p) or 2 (Sub 960x480/576).
func ValidateStreamType(streamType int) error {
	if streamType != StreamTypeMain && streamType != StreamTypeSub {
		return ErrInvalidStreamType
	}
	return nil
}

// TrackID computes the Hikvision/ISAPI track/stream ID (e.g. channel 1 + main stream 1 -> 101; sub stream 2 -> 102).
func TrackID(channelID, streamType int) (int, error) {
	if err := ValidateChannelID(channelID); err != nil {
		return 0, err
	}
	if err := ValidateStreamType(streamType); err != nil {
		return 0, err
	}
	return channelID*100 + streamType, nil
}

// DeviceInfo represents the TV-DVR208 hardware and firmware metadata.
type DeviceInfo struct {
	DeviceName           string `json:"deviceName"`
	DeviceID             string `json:"deviceId"`
	Model                string `json:"model"`
	SerialNumber         string `json:"serialNumber"`
	MACAddress           string `json:"macAddress"`
	FirmwareVersion      string `json:"firmwareVersion"`
	FirmwareReleasedDate string `json:"firmwareReleasedDate"`
	EncoderVersion       string `json:"encoderVersion"`
	EncoderReleasedDate  string `json:"encoderReleasedDate"`
	DeviceType           string `json:"deviceType"`
	TelecontrolID        int    `json:"telecontrolId"`
}

// SystemTime represents the DVR system clock and NTP mode.
type SystemTime struct {
	TimeMode  string `json:"timeMode"`
	LocalTime string `json:"localTime"`
	TimeZone  string `json:"timeZone"`
}

// StorageVolume represents an internal SATA HDD on the DVR.
type StorageVolume struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	Status      string `json:"status"`
	CapacityMB  int64  `json:"capacityMb"`
	FreeSpaceMB int64  `json:"freeSpaceMb"`
	Property    string `json:"property"`
}

// NetworkInterface represents the DVR LAN configuration.
type NetworkInterface struct {
	ID             int    `json:"id"`
	AddressingType string `json:"addressingType"`
	IPAddress      string `json:"ipAddress"`
	SubnetMask     string `json:"subnetMask"`
	DefaultGateway string `json:"defaultGateway"`
	PrimaryDNS     string `json:"primaryDns"`
	SecondaryDNS   string `json:"secondaryDns"`
	MACAddress     string `json:"macAddress"`
	MTU            int    `json:"mtu"`
}

// UserAccount represents an operator or administrator user configured on the DVR.
type UserAccount struct {
	ID        int    `json:"id"`
	UserName  string `json:"userName"`
	UserLevel string `json:"userLevel"`
}

// VideoChannel represents an input camera channel and its current encoding parameters.
type VideoChannel struct {
	ID          int           `json:"id"`
	InputPort   int           `json:"inputPort"`
	Enabled     bool          `json:"enabled"`
	Name        string        `json:"name"`
	VideoFormat string        `json:"videoFormat"`
	ResDesc     string        `json:"resDesc"`
	MainStream  *StreamConfig `json:"mainStream,omitempty"`
	SubStream   *StreamConfig `json:"subStream,omitempty"`
}

// StreamConfig represents the H.264 video/audio encoding parameters of a track (e.g. 101 or 102).
type StreamConfig struct {
	TrackID            int    `json:"trackId"`
	ChannelID          int    `json:"channelId"`
	Enabled            bool   `json:"enabled"`
	VideoCodecType     string `json:"videoCodecType"`
	ResolutionWidth    int    `json:"resolutionWidth"`
	ResolutionHeight   int    `json:"resolutionHeight"`
	QualityControlType string `json:"qualityControlType"`
	MaxBitrateKbps     int    `json:"maxBitrateKbps"`
	MaxFrameRate       int    `json:"maxFrameRate"` // ISAPI stores fps*100 (e.g. 2500 = 25fps, 1200 = 12fps)
	AudioEnabled       bool   `json:"audioEnabled"`
	AudioCodecType     string `json:"audioCodecType"`
}

// FrameRateFPS returns the human-readable frame rate in frames per second.
func (s StreamConfig) FrameRateFPS() float64 {
	if s.MaxFrameRate >= 100 {
		return float64(s.MaxFrameRate) / 100.0
	}
	return float64(s.MaxFrameRate)
}

// RecordingDaySpan indicates whether a given day of a month has archived recordings.
type RecordingDaySpan struct {
	DayNumber  int    `json:"dayNumber"`
	HasRecords bool   `json:"hasRecords"`
	RecordType string `json:"recordType,omitempty"`
}

// RecordingSegment represents an archived recording file/span returned by ISAPI ContentMgmt/search.
type RecordingSegment struct {
	TrackID     int       `json:"trackId"`
	ChannelID   int       `json:"channelId"`
	StartTime   time.Time `json:"startTime"`
	EndTime     time.Time `json:"endTime"`
	PlaybackURI string    `json:"playbackUri"`
	FileName    string    `json:"fileName"`
	SizeBytes   int64     `json:"sizeBytes"`
	ContentType string    `json:"contentType"`
}

// RecordingSearchQuery specifies parameters for searching archived recordings.
type RecordingSearchQuery struct {
	ChannelID int
	StartTime time.Time
	EndTime   time.Time
}

func (q RecordingSearchQuery) Validate() error {
	if err := ValidateChannelID(q.ChannelID); err != nil {
		return err
	}
	if !q.StartTime.Before(q.EndTime) {
		return ErrInvalidTimeRange
	}
	return nil
}

// PTZCommand represents a continuous pan/tilt/zoom movement vector (-100..100).
type PTZCommand struct {
	Pan  int `json:"pan"`
	Tilt int `json:"tilt"`
	Zoom int `json:"zoom"`
}

func (c PTZCommand) Validate() error {
	if c.Pan < -100 || c.Pan > 100 || c.Tilt < -100 || c.Tilt > 100 || c.Zoom < -100 || c.Zoom > 100 {
		return ErrInvalidPTZSpeed
	}
	return nil
}

// PTZPosition3D represents a 3D drag-to-zoom selection box on the 255x255 or 704x576 canvas.
type PTZPosition3D struct {
	StartX int `json:"startX"`
	StartY int `json:"startY"`
	EndX   int `json:"endX"`
	EndY   int `json:"endY"`
}

// TextOverlay represents a custom OSD text line on the 704x576 normalized screen.
type TextOverlay struct {
	ID          int    `json:"id"`
	Enabled     bool   `json:"enabled"`
	PositionX   int    `json:"positionX"`
	PositionY   int    `json:"positionY"`
	DisplayText string `json:"displayText"`
}

// DateTimeOverlay represents the timestamp OSD overlay.
type DateTimeOverlay struct {
	Enabled     bool   `json:"enabled"`
	PositionX   int    `json:"positionX"`
	PositionY   int    `json:"positionY"`
	DateStyle   string `json:"dateStyle"`
	TimeStyle   string `json:"timeStyle"`
	DisplayWeek bool   `json:"displayWeek"`
}

// ChannelNameOverlay represents the camera title OSD overlay.
type ChannelNameOverlay struct {
	Enabled   bool `json:"enabled"`
	PositionX int  `json:"positionX"`
	PositionY int  `json:"positionY"`
}

// VideoOverlayConfig aggregates all OSD settings for a camera channel.
type VideoOverlayConfig struct {
	ChannelID      int                `json:"channelId"`
	NormalizedW    int                `json:"normalizedWidth"`
	NormalizedH    int                `json:"normalizedHeight"`
	Transparent    bool               `json:"transparent"`
	Flashing       bool               `json:"flashing"`
	FontSize       int                `json:"fontSize"`
	DateTime       DateTimeOverlay    `json:"dateTime"`
	ChannelName    ChannelNameOverlay `json:"channelName"`
	TextOverlays   []TextOverlay      `json:"textOverlays"`
}

func (v VideoOverlayConfig) Validate() error {
	if v.DateTime.PositionX < 0 || v.DateTime.PositionX > NormalizedWidth ||
		v.DateTime.PositionY < 0 || v.DateTime.PositionY > NormalizedHeight {
		return ErrInvalidOverlayCoord
	}
	if v.ChannelName.PositionX < 0 || v.ChannelName.PositionX > NormalizedWidth ||
		v.ChannelName.PositionY < 0 || v.ChannelName.PositionY > NormalizedHeight {
		return ErrInvalidOverlayCoord
	}
	return nil
}

// MotionGrid represents the 18x22 boolean grid used for Hikvision motion detection.
type MotionGrid [MotionRows][MotionCols]bool

// ParseMotionGridHex decodes Hikvision's 108-character hex gridMap (6 hex chars = 24 bits per row,
// where column c in 0..21 is stored at bit 23-c).
func ParseMotionGridHex(gridMapHex string) (MotionGrid, error) {
	var grid MotionGrid
	cleaned := strings.TrimSpace(gridMapHex)
	if cleaned == "" {
		return grid, nil
	}
	if len(cleaned) != MotionRows*6 {
		return grid, fmt.Errorf("%w: expected %d hex chars, got %d", ErrInvalidMotionGrid, MotionRows*6, len(cleaned))
	}
	for r := 0; r < MotionRows; r++ {
		rowHex := cleaned[r*6 : (r+1)*6]
		val, err := strconv.ParseUint(rowHex, 16, 32)
		if err != nil {
			return grid, fmt.Errorf("%w: row %d invalid hex %q: %v", ErrInvalidMotionGrid, r, rowHex, err)
		}
		for c := 0; c < MotionCols; c++ {
			bit := uint(23 - c)
			grid[r][c] = ((val >> bit) & 1) == 1
		}
	}
	return grid, nil
}

// ToHex encodes the 18x22 boolean grid into Hikvision's 108-character hex gridMap.
func (g MotionGrid) ToHex() string {
	var sb strings.Builder
	sb.Grow(MotionRows * 6)
	for r := 0; r < MotionRows; r++ {
		var rowVal uint32
		for c := 0; c < MotionCols; c++ {
			if g[r][c] {
				rowVal |= 1 << uint(23-c)
			}
		}
		sb.WriteString(fmt.Sprintf("%06x", rowVal))
	}
	return sb.String()
}

// MotionDetectionConfig represents motion detection parameters for a channel.
type MotionDetectionConfig struct {
	ChannelID          int        `json:"channelId"`
	Enabled            bool       `json:"enabled"`
	EnableHighlight    bool       `json:"enableHighlight"`
	SamplingInterval   int        `json:"samplingInterval"`
	StartTriggerTimeMs int        `json:"startTriggerTimeMs"`
	EndTriggerTimeMs   int        `json:"endTriggerTimeMs"`
	SensitivityLevel   int        `json:"sensitivityLevel"`
	RowGranularity     int        `json:"rowGranularity"`
	ColumnGranularity  int        `json:"columnGranularity"`
	GridMapHex         string     `json:"gridMapHex"`
	Grid               MotionGrid `json:"grid"`
}

// PrivacyMaskRegion represents a rectangular privacy mask on the 704x576 normalized screen.
type PrivacyMaskRegion struct {
	ID      int  `json:"id"`
	Enabled bool `json:"enabled"`
	X       int  `json:"x"`
	Y       int  `json:"y"`
	Width   int  `json:"width"`
	Height  int  `json:"height"`
}

// PrivacyMaskConfig represents privacy masking configuration for a channel.
type PrivacyMaskConfig struct {
	ChannelID   int                 `json:"channelId"`
	Enabled     bool                `json:"enabled"`
	NormalizedW int                 `json:"normalizedWidth"`
	NormalizedH int                 `json:"normalizedHeight"`
	Regions     []PrivacyMaskRegion `json:"regions"`
}
