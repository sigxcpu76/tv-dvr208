package isapi

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/sigxcpu/tv-dvr208/backend/internal/domain"
)

// Client is the driven ISAPI HTTP/XML adapter for the TV-DVR208.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient creates a new ISAPI client configured with RFC 2617 HTTP Digest Authentication.
func NewClient(baseURL, username, password string, timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: timeout,
			Transport: &DigestTransport{
				Username: username,
				Password: password,
			},
		},
	}
}

func (c *Client) doXML(ctx context.Context, method, path string, reqBody []byte, out any) error {
	var bodyReader io.Reader
	if reqBody != nil {
		bodyReader = bytes.NewReader(reqBody)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bodyReader)
	if err != nil {
		return err
	}
	if reqBody != nil {
		req.Header.Set("Content-Type", "application/xml; charset=utf-8")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %v", domain.ErrUpstreamFailure, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return domain.ErrAuthentication
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("%w: HTTP %d on %s %s: %s", domain.ErrUpstreamFailure, resp.StatusCode, method, path, string(data))
	}

	if out != nil && len(data) > 0 {
		if err := xml.Unmarshal(data, out); err != nil {
			return fmt.Errorf("failed to unmarshal XML from %s: %w", path, err)
		}
	}
	return nil
}

// --- DVRSystemPort ---

func (c *Client) GetDeviceInfo(ctx context.Context) (*domain.DeviceInfo, error) {
	var raw xmlDeviceInfo
	if err := c.doXML(ctx, http.MethodGet, "/ISAPI/System/deviceInfo", nil, &raw); err != nil {
		return nil, err
	}
	return &domain.DeviceInfo{
		DeviceName:           raw.DeviceName,
		DeviceID:             raw.DeviceID,
		Model:                raw.Model,
		SerialNumber:         raw.SerialNumber,
		MACAddress:           raw.MACAddress,
		FirmwareVersion:      raw.FirmwareVersion,
		FirmwareReleasedDate: raw.FirmwareReleasedDate,
		EncoderVersion:       raw.EncoderVersion,
		EncoderReleasedDate:  raw.EncoderReleasedDate,
		DeviceType:           raw.DeviceType,
		TelecontrolID:        raw.TelecontrolID,
	}, nil
}

func (c *Client) GetSystemTime(ctx context.Context) (*domain.SystemTime, error) {
	var raw xmlSystemTime
	if err := c.doXML(ctx, http.MethodGet, "/ISAPI/System/time", nil, &raw); err != nil {
		return nil, err
	}
	return &domain.SystemTime{
		TimeMode:  raw.TimeMode,
		LocalTime: raw.LocalTime,
		TimeZone:  raw.TimeZone,
	}, nil
}

func (c *Client) GetStorageVolumes(ctx context.Context) ([]domain.StorageVolume, error) {
	var raw xmlHDDList
	if err := c.doXML(ctx, http.MethodGet, "/ISAPI/ContentMgmt/Storage/hdd", nil, &raw); err != nil {
		return nil, err
	}
	vols := make([]domain.StorageVolume, 0, len(raw.HDDs))
	for _, h := range raw.HDDs {
		vols = append(vols, domain.StorageVolume{
			ID:          h.ID,
			Name:        h.HDDName,
			Type:        h.HDDType,
			Status:      h.Status,
			CapacityMB:  h.Capacity,
			FreeSpaceMB: h.FreeSpace,
			Property:    h.Property,
		})
	}
	return vols, nil
}

func (c *Client) GetNetworkInterface(ctx context.Context) (*domain.NetworkInterface, error) {
	var raw xmlNetworkInterface
	if err := c.doXML(ctx, http.MethodGet, "/ISAPI/System/Network/interfaces/1", nil, &raw); err != nil {
		return nil, err
	}
	return &domain.NetworkInterface{
		ID:             raw.ID,
		AddressingType: raw.IPAddress.AddressingType,
		IPAddress:      raw.IPAddress.IPAddress,
		SubnetMask:     raw.IPAddress.SubnetMask,
		DefaultGateway: raw.IPAddress.DefaultGateway.IPAddress,
		PrimaryDNS:     raw.IPAddress.PrimaryDNS.IPAddress,
		SecondaryDNS:   raw.IPAddress.SecondaryDNS.IPAddress,
		MACAddress:     raw.Link.MACAddress,
		MTU:            raw.Link.MTU,
	}, nil
}

func (c *Client) GetUsers(ctx context.Context) ([]domain.UserAccount, error) {
	var raw xmlUserList
	if err := c.doXML(ctx, http.MethodGet, "/ISAPI/Security/users", nil, &raw); err != nil {
		return nil, err
	}
	users := make([]domain.UserAccount, 0, len(raw.Users))
	for _, u := range raw.Users {
		users = append(users, domain.UserAccount{
			ID:        u.ID,
			UserName:  u.UserName,
			UserLevel: u.UserLevel,
		})
	}
	return users, nil
}

func (c *Client) RebootDevice(ctx context.Context) error {
	return c.doXML(ctx, http.MethodPut, "/ISAPI/System/reboot", nil, nil)
}

// --- DVRChannelPort ---

func (c *Client) ListVideoChannels(ctx context.Context) ([]domain.VideoChannel, error) {
	var raw xmlVideoInputChannelList
	if err := c.doXML(ctx, http.MethodGet, "/ISAPI/System/Video/inputs/channels", nil, &raw); err != nil {
		return nil, err
	}
	channels := make([]domain.VideoChannel, 0, len(raw.Channels))
	for _, ch := range raw.Channels {
		channels = append(channels, domain.VideoChannel{
			ID:          ch.ID,
			InputPort:   ch.InputPort,
			Enabled:     ch.VideoInputEnabled,
			Name:        ch.Name,
			VideoFormat: ch.VideoFormat,
			ResDesc:     ch.ResDesc,
		})
	}
	return channels, nil
}

func (c *Client) GetStreamConfig(ctx context.Context, trackID int) (*domain.StreamConfig, error) {
	var raw xmlStreamingChannel
	path := fmt.Sprintf("/ISAPI/Streaming/channels/%d", trackID)
	if err := c.doXML(ctx, http.MethodGet, path, nil, &raw); err != nil {
		return nil, err
	}
	bitrate := raw.Video.VBRUpperCap
	if bitrate == 0 {
		bitrate = raw.Video.ConstantBitRate
	}
	return &domain.StreamConfig{
		TrackID:            raw.ID,
		ChannelID:          raw.Video.VideoInputChannelID,
		Enabled:            raw.Enabled,
		VideoCodecType:     raw.Video.VideoCodecType,
		ResolutionWidth:    raw.Video.VideoResolutionWidth,
		ResolutionHeight:   raw.Video.VideoResolutionHeight,
		QualityControlType: raw.Video.VideoQualityControlType,
		MaxBitrateKbps:     bitrate,
		MaxFrameRate:       raw.Video.MaxFrameRate,
		AudioEnabled:       raw.Audio.Enabled,
		AudioCodecType:     raw.Audio.AudioCompressionType,
	}, nil
}

func (c *Client) CaptureSnapshot(ctx context.Context, trackID int) ([]byte, error) {
	path := fmt.Sprintf("/ISAPI/Streaming/channels/%d/picture", trackID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", domain.ErrUpstreamFailure, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, domain.ErrAuthentication
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%w: snapshot HTTP %d", domain.ErrUpstreamFailure, resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

func (c *Client) GetVideoOverlays(ctx context.Context, channelID int) (*domain.VideoOverlayConfig, error) {
	var raw xmlVideoOverlay
	path := fmt.Sprintf("/ISAPI/System/Video/inputs/channels/%d/overlays", channelID)
	if err := c.doXML(ctx, http.MethodGet, path, nil, &raw); err != nil {
		return nil, err
	}
	texts := make([]domain.TextOverlay, 0, len(raw.TextOverlayList))
	for _, t := range raw.TextOverlayList {
		texts = append(texts, domain.TextOverlay{
			ID:          t.ID,
			Enabled:     t.Enabled,
			PositionX:   int(t.PositionX),
			PositionY:   int(t.PositionY),
			DisplayText: t.DisplayText,
		})
	}
	normW := raw.NormalizedScreenSize.Width
	if normW == 0 {
		normW = domain.NormalizedWidth
	}
	normH := raw.NormalizedScreenSize.Height
	if normH == 0 {
		normH = domain.NormalizedHeight
	}
	return &domain.VideoOverlayConfig{
		ChannelID:   channelID,
		NormalizedW: normW,
		NormalizedH: normH,
		Transparent: raw.Attribute.Transparent,
		Flashing:    raw.Attribute.Flashing,
		FontSize:    raw.FontSize,
		DateTime: domain.DateTimeOverlay{
			Enabled:     raw.DateTimeOverlay.Enabled,
			PositionX:   raw.DateTimeOverlay.PositionX,
			PositionY:   raw.DateTimeOverlay.PositionY,
			DateStyle:   raw.DateTimeOverlay.DateStyle,
			TimeStyle:   raw.DateTimeOverlay.TimeStyle,
			DisplayWeek: raw.DateTimeOverlay.DisplayWeek,
		},
		ChannelName: domain.ChannelNameOverlay{
			Enabled:   raw.ChannelNameOverlay.Enabled,
			PositionX: raw.ChannelNameOverlay.PositionX,
			PositionY: raw.ChannelNameOverlay.PositionY,
		},
		TextOverlays: texts,
	}, nil
}

func (c *Client) UpdateVideoOverlays(ctx context.Context, channelID int, cfg domain.VideoOverlayConfig) error {
	dateStyle := cfg.DateTime.DateStyle
	if dateStyle == "" {
		dateStyle = "YYYY-MM-DD"
	}
	timeStyle := cfg.DateTime.TimeStyle
	if timeStyle == "" {
		timeStyle = "24hour"
	}
	fontSize := cfg.FontSize
	if fontSize <= 0 {
		fontSize = 1
	}

	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	sb.WriteString(`<VideoOverlay version="1.0" xmlns="http://www.std-cgi.com/ver20/XMLSchema">`)
	sb.WriteString(fmt.Sprintf(`<normalizedScreenSize><normalizedScreenWidth>%d</normalizedScreenWidth><normalizedScreenHeight>%d</normalizedScreenHeight></normalizedScreenSize>`, domain.NormalizedWidth, domain.NormalizedHeight))
	sb.WriteString(fmt.Sprintf(`<attribute><transparent>%t</transparent><flashing>%t</flashing></attribute>`, cfg.Transparent, cfg.Flashing))
	sb.WriteString(fmt.Sprintf(`<fontSize>%d</fontSize>`, fontSize))
	sb.WriteString(`<TextOverlayList>`)
	for _, t := range cfg.TextOverlays {
		sb.WriteString(fmt.Sprintf(`<TextOverlay version="1.0" xmlns="http://www.std-cgi.com/ver20/XMLSchema"><id>%d</id><enabled>%t</enabled><positionX>%d</positionX><positionY>%d</positionY><displayText>%s</displayText></TextOverlay>`,
			t.ID, t.Enabled, t.PositionX, t.PositionY, escapeXML(t.DisplayText)))
	}
	sb.WriteString(`</TextOverlayList>`)
	sb.WriteString(fmt.Sprintf(`<DateTimeOverlay version="1.0" xmlns="http://www.std-cgi.com/ver20/XMLSchema"><enabled>%t</enabled><positionX>%d</positionX><positionY>%d</positionY><dateStyle>%s</dateStyle><timeStyle>%s</timeStyle><displayWeek>%t</displayWeek></DateTimeOverlay>`,
		cfg.DateTime.Enabled, cfg.DateTime.PositionX, cfg.DateTime.PositionY, escapeXML(dateStyle), escapeXML(timeStyle), cfg.DateTime.DisplayWeek))
	sb.WriteString(fmt.Sprintf(`<channelNameOverlay version="1.0" xmlns="http://www.std-cgi.com/ver20/XMLSchema"><enabled>%t</enabled><positionX>%d</positionX><positionY>%d</positionY></channelNameOverlay>`,
		cfg.ChannelName.Enabled, cfg.ChannelName.PositionX, cfg.ChannelName.PositionY))
	sb.WriteString(`</VideoOverlay>`)

	path := fmt.Sprintf("/ISAPI/System/Video/inputs/channels/%d/overlays", channelID)
	return c.doXML(ctx, http.MethodPut, path, []byte(sb.String()), nil)
}

func (c *Client) GetMotionDetection(ctx context.Context, channelID int) (*domain.MotionDetectionConfig, error) {
	var raw xmlMotionDetection
	path := fmt.Sprintf("/ISAPI/System/Video/inputs/channels/%d/motionDetection", channelID)
	if err := c.doXML(ctx, http.MethodGet, path, nil, &raw); err != nil {
		return nil, err
	}
	grid, _ := domain.ParseMotionGridHex(raw.MotionDetectionLayout.Layout.GridMap)
	rows := raw.Grid.RowGranularity
	if rows == 0 {
		rows = domain.MotionRows
	}
	cols := raw.Grid.ColumnGranularity
	if cols == 0 {
		cols = domain.MotionCols
	}
	return &domain.MotionDetectionConfig{
		ChannelID:          channelID,
		Enabled:            raw.Enabled,
		EnableHighlight:    raw.EnableHighlight,
		SamplingInterval:   raw.SamplingInterval,
		StartTriggerTimeMs: raw.StartTriggerTime,
		EndTriggerTimeMs:   raw.EndTriggerTime,
		SensitivityLevel:   raw.MotionDetectionLayout.SensitivityLevel,
		RowGranularity:     rows,
		ColumnGranularity:  cols,
		GridMapHex:         raw.MotionDetectionLayout.Layout.GridMap,
		Grid:               grid,
	}, nil
}

func (c *Client) UpdateMotionDetection(ctx context.Context, channelID int, cfg domain.MotionDetectionConfig) error {
	sampling := cfg.SamplingInterval
	if sampling <= 0 {
		sampling = 5
	}
	startTrig := cfg.StartTriggerTimeMs
	if startTrig <= 0 {
		startTrig = 1000
	}
	endTrig := cfg.EndTriggerTimeMs
	if endTrig <= 0 {
		endTrig = 1000
	}
	gridHex := cfg.GridMapHex
	if gridHex == "" {
		gridHex = cfg.Grid.ToHex()
	}

	payload := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>`+
		`<MotionDetection version="1.0" xmlns="http://www.std-cgi.com/ver20/XMLSchema">`+
		`<enabled>%t</enabled>`+
		`<enableHighlight>%t</enableHighlight>`+
		`<samplingInterval>%d</samplingInterval>`+
		`<startTriggerTime>%d</startTriggerTime>`+
		`<endTriggerTime>%d</endTriggerTime>`+
		`<regionType>grid</regionType>`+
		`<Grid><rowGranularity>%d</rowGranularity><columnGranularity>%d</columnGranularity></Grid>`+
		`<MotionDetectionLayout version="1.0" xmlns="http://www.std-cgi.com/ver20/XMLSchema">`+
		`<sensitivityLevel>%d</sensitivityLevel>`+
		`<FactoryProtocol>HIKVISION</FactoryProtocol>`+
		`<layout><gridMap>%s</gridMap></layout>`+
		`</MotionDetectionLayout>`+
		`</MotionDetection>`,
		cfg.Enabled, cfg.EnableHighlight, sampling, startTrig, endTrig,
		domain.MotionRows, domain.MotionCols, cfg.SensitivityLevel, escapeXML(gridHex))

	path := fmt.Sprintf("/ISAPI/System/Video/inputs/channels/%d/motionDetection", channelID)
	return c.doXML(ctx, http.MethodPut, path, []byte(payload), nil)
}

func (c *Client) GetPrivacyMask(ctx context.Context, channelID int) (*domain.PrivacyMaskConfig, error) {
	var raw xmlPrivacyMask
	path := fmt.Sprintf("/ISAPI/System/Video/inputs/channels/%d/privacyMask", channelID)
	if err := c.doXML(ctx, http.MethodGet, path, nil, &raw); err != nil {
		return nil, err
	}
	regions := make([]domain.PrivacyMaskRegion, 0, len(raw.Regions))
	for _, r := range raw.Regions {
		minX, minY, maxX, maxY := 0, 0, 0, 0
		for i, pt := range r.CoordinatesList {
			if i == 0 || pt.PositionX < minX {
				minX = pt.PositionX
			}
			if i == 0 || pt.PositionY < minY {
				minY = pt.PositionY
			}
			if i == 0 || pt.PositionX > maxX {
				maxX = pt.PositionX
			}
			if i == 0 || pt.PositionY > maxY {
				maxY = pt.PositionY
			}
		}
		regions = append(regions, domain.PrivacyMaskRegion{
			ID:      r.ID,
			Enabled: r.Enabled,
			X:       minX,
			Y:       minY,
			Width:   maxX - minX,
			Height:  maxY - minY,
		})
	}
	return &domain.PrivacyMaskConfig{
		ChannelID:   channelID,
		Enabled:     raw.Enabled,
		NormalizedW: domain.NormalizedWidth,
		NormalizedH: domain.NormalizedHeight,
		Regions:     regions,
	}, nil
}

func (c *Client) UpdatePrivacyMask(ctx context.Context, channelID int, cfg domain.PrivacyMaskConfig) error {
	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	sb.WriteString(`<PrivacyMask version="1.0" xmlns="http://www.std-cgi.com/ver20/XMLSchema">`)
	sb.WriteString(fmt.Sprintf(`<enabled>%t</enabled>`, cfg.Enabled))
	sb.WriteString(fmt.Sprintf(`<normalizedScreenSize><normalizedScreenWidth>%d</normalizedScreenWidth><normalizedScreenHeight>%d</normalizedScreenHeight></normalizedScreenSize>`, domain.NormalizedWidth, domain.NormalizedHeight))
	sb.WriteString(`<PrivacyMaskRegionList version="1.0" xmlns="http://www.std-cgi.com/ver20/XMLSchema">`)
	for _, r := range cfg.Regions {
		x1, y1 := r.X, r.Y
		x2, y2 := r.X+r.Width, r.Y+r.Height
		sb.WriteString(fmt.Sprintf(`<PrivacyMaskRegion version="1.0" xmlns="http://www.std-cgi.com/ver20/XMLSchema"><id>%d</id><enabled>%t</enabled><RegionCoordinatesList version="1.0" xmlns="http://www.std-cgi.com/ver20/XMLSchema">`+
			`<RegionCoordinates><positionX>%d</positionX><positionY>%d</positionY></RegionCoordinates>`+
			`<RegionCoordinates><positionX>%d</positionX><positionY>%d</positionY></RegionCoordinates>`+
			`<RegionCoordinates><positionX>%d</positionX><positionY>%d</positionY></RegionCoordinates>`+
			`<RegionCoordinates><positionX>%d</positionX><positionY>%d</positionY></RegionCoordinates>`+
			`</RegionCoordinatesList></PrivacyMaskRegion>`,
			r.ID, r.Enabled, x1, y1, x2, y1, x2, y2, x1, y2))
	}
	sb.WriteString(`</PrivacyMaskRegionList></PrivacyMask>`)

	path := fmt.Sprintf("/ISAPI/System/Video/inputs/channels/%d/privacyMask", channelID)
	return c.doXML(ctx, http.MethodPut, path, []byte(sb.String()), nil)
}

// --- DVRPTZPort ---

func (c *Client) ContinuousMove(ctx context.Context, channelID int, cmd domain.PTZCommand) error {
	payload := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?><PTZData><pan>%d</pan><tilt>%d</tilt><zoom>%d</zoom></PTZData>`,
		cmd.Pan, cmd.Tilt, cmd.Zoom)
	path := fmt.Sprintf("/ISAPI/PTZCtrl/channels/%d/continuous", channelID)
	return c.doXML(ctx, http.MethodPut, path, []byte(payload), nil)
}

func (c *Client) GotoPreset(ctx context.Context, channelID int, presetID int) error {
	path := fmt.Sprintf("/ISAPI/PTZCtrl/channels/%d/presets/%d/goto", channelID, presetID)
	return c.doXML(ctx, http.MethodPut, path, nil, nil)
}

func (c *Client) SetPreset(ctx context.Context, channelID int, presetID int) error {
	payload := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?><PTZPreset><id>%d</id><presetName>Preset %d</presetName></PTZPreset>`, presetID, presetID)
	path := fmt.Sprintf("/ISAPI/PTZCtrl/channels/%d/presets/%d", channelID, presetID)
	return c.doXML(ctx, http.MethodPut, path, []byte(payload), nil)
}

func (c *Client) Zoom3D(ctx context.Context, channelID int, pos domain.PTZPosition3D) error {
	payload := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?><position3D><StartPoint><positionX>%d</positionX><positionY>%d</positionY></StartPoint><EndPoint><positionX>%d</positionX><positionY>%d</positionY></EndPoint></position3D>`,
		pos.StartX, pos.StartY, pos.EndX, pos.EndY)
	path := fmt.Sprintf("/ISAPI/PTZCtrl/channels/%d/position3D", channelID)
	return c.doXML(ctx, http.MethodPut, path, []byte(payload), nil)
}

// --- DVRArchivePort ---

func (c *Client) GetDailyDistribution(ctx context.Context, trackID int, year int, month int) ([]domain.RecordingDaySpan, error) {
	payload := fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?><trackDailyParam><year>%d</year><monthOfYear>%d</monthOfYear></trackDailyParam>`, year, month)
	path := fmt.Sprintf("/ISAPI/ContentMgmt/record/tracks/%d/dailyDistribution", trackID)
	var raw xmlTrackDailyDistribution
	if err := c.doXML(ctx, http.MethodPost, path, []byte(payload), &raw); err != nil {
		return nil, err
	}
	days := make([]domain.RecordingDaySpan, 0, len(raw.Days))
	for _, d := range raw.Days {
		days = append(days, domain.RecordingDaySpan{
			DayNumber:  d.DayOfMonth,
			HasRecords: d.Record,
			RecordType: d.RecordType,
		})
	}
	return days, nil
}

func (c *Client) SearchRecordings(ctx context.Context, trackID int, startTime, endTime time.Time) ([]domain.RecordingSegment, error) {
	const maxResults = 50
	var allSegments []domain.RecordingSegment
	position := 0

	for page := 0; page < 10; page++ {
		payload := fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?>`+
			`<CMSearchDescription>`+
			`<searchID>C0000001-0000-0000-0000-%012d</searchID>`+
			`<trackList><trackID>%d</trackID></trackList>`+
			`<timeSpanList><timeSpan><startTime>%s</startTime><endTime>%s</endTime></timeSpan></timeSpanList>`+
			`<maxResults>%d</maxResults>`+
			`<searchResultPostion>%d</searchResultPostion>`+
			`<metadataList><metadataDescriptor>//recordType.meta.std-cgi.com</metadataDescriptor></metadataList>`+
			`</CMSearchDescription>`,
			page+1,
			trackID,
			startTime.UTC().Format("2006-01-02T15:04:05Z"),
			endTime.UTC().Format("2006-01-02T15:04:05Z"),
			maxResults,
			position,
		)

		var raw xmlCMSearchResult
		if err := c.doXML(ctx, http.MethodPost, "/ISAPI/ContentMgmt/search", []byte(payload), &raw); err != nil {
			return nil, err
		}

		for _, m := range raw.Matches {
			st, _ := time.Parse(time.RFC3339, m.TimeSpan.StartTime)
			et, _ := time.Parse(time.RFC3339, m.TimeSpan.EndTime)
			name, sizeBytes := parsePlaybackURIParams(m.MediaSegmentDescriptor.PlaybackURI)
			allSegments = append(allSegments, domain.RecordingSegment{
				TrackID:     m.TrackID,
				ChannelID:   m.TrackID / 100,
				StartTime:   st,
				EndTime:     et,
				PlaybackURI: m.MediaSegmentDescriptor.PlaybackURI,
				FileName:    name,
				SizeBytes:   sizeBytes,
				ContentType: m.MediaSegmentDescriptor.ContentType,
			})
		}

		if strings.ToUpper(raw.ResponseStatusStrg) != "MORE" || len(raw.Matches) == 0 {
			break
		}
		position += len(raw.Matches)
	}
	return allSegments, nil
}

func (c *Client) OpenRecordingDownloadStream(ctx context.Context, playbackURI string) (io.ReadCloser, error) {
	payload := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?><downloadRequest version="1.0" xmlns="http://www.isapi.org/ver20/XMLSchema"><playbackURI>%s</playbackURI></downloadRequest>`,
		escapeXML(playbackURI))

	// Use a dedicated HTTP client without a short wall-clock timeout for long recording streams
	streamClient := &http.Client{
		Transport: c.httpClient.Transport,
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/ISAPI/ContentMgmt/download", strings.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/xml; charset=utf-8")

	resp, err := streamClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", domain.ErrUpstreamFailure, err)
	}
	if resp.StatusCode == http.StatusUnauthorized {
		resp.Body.Close()
		return nil, domain.ErrAuthentication
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("%w: download HTTP %d: %s", domain.ErrUpstreamFailure, resp.StatusCode, string(body))
	}
	return resp.Body, nil
}

func parsePlaybackURIParams(uriStr string) (string, int64) {
	u, err := url.Parse(uriStr)
	if err != nil {
		return "", 0
	}
	q := u.Query()
	name := q.Get("name")
	sizeVal, _ := strconv.ParseInt(q.Get("size"), 10, 64)
	return name, sizeVal
}

func escapeXML(s string) string {
	var buf bytes.Buffer
	_ = xml.EscapeText(&buf, []byte(s))
	return buf.String()
}
