package isapi

import "encoding/xml"

const isapiXMLNS = "http://www.std-cgi.com/ver20/XMLSchema"

type xmlDeviceInfo struct {
	XMLName              xml.Name `xml:"DeviceInfo"`
	DeviceName           string   `xml:"deviceName"`
	DeviceID             string   `xml:"deviceID"`
	Model                string   `xml:"model"`
	SerialNumber         string   `xml:"serialNumber"`
	MACAddress           string   `xml:"macAddress"`
	FirmwareVersion      string   `xml:"firmwareVersion"`
	FirmwareReleasedDate string   `xml:"firmwareReleasedDate"`
	EncoderVersion       string   `xml:"encoderVersion"`
	EncoderReleasedDate  string   `xml:"encoderReleasedDate"`
	DeviceType           string   `xml:"deviceType"`
	TelecontrolID        int      `xml:"telecontrolID"`
}

type xmlSystemTime struct {
	XMLName   xml.Name `xml:"Time"`
	TimeMode  string   `xml:"timeMode"`
	LocalTime string   `xml:"localTime"`
	TimeZone  string   `xml:"timeZone"`
}

type xmlHDDList struct {
	XMLName xml.Name `xml:"hddList"`
	HDDs    []xmlHDD `xml:"hdd"`
}

type xmlHDD struct {
	ID        int    `xml:"id"`
	HDDName   string `xml:"hddName"`
	HDDType   string `xml:"hddType"`
	Status    string `xml:"status"`
	Capacity  int64  `xml:"capacity"`
	FreeSpace int64  `xml:"freeSpace"`
	Property  string `xml:"property"`
}

type xmlNetworkInterface struct {
	XMLName   xml.Name `xml:"NetworkInterface"`
	ID        int      `xml:"id"`
	IPAddress struct {
		AddressingType string `xml:"addressingType"`
		IPAddress      string `xml:"ipAddress"`
		SubnetMask     string `xml:"subnetMask"`
		DefaultGateway struct {
			IPAddress string `xml:"ipAddress"`
		} `xml:"DefaultGateway"`
		PrimaryDNS struct {
			IPAddress string `xml:"ipAddress"`
		} `xml:"PrimaryDNS"`
		SecondaryDNS struct {
			IPAddress string `xml:"ipAddress"`
		} `xml:"SecondaryDNS"`
	} `xml:"IPAddress"`
	Link struct {
		MACAddress string `xml:"MACAddress"`
		MTU        int    `xml:"MTU"`
	} `xml:"Link"`
}

type xmlUserList struct {
	XMLName xml.Name  `xml:"UserList"`
	Users   []xmlUser `xml:"User"`
}

type xmlUser struct {
	ID        int    `xml:"id"`
	UserName  string `xml:"userName"`
	UserLevel string `xml:"userLevel"`
}

type xmlVideoInputChannelList struct {
	XMLName  xml.Name               `xml:"VideoInputChannelList"`
	Channels []xmlVideoInputChannel `xml:"VideoInputChannel"`
}

type xmlVideoInputChannel struct {
	ID                int    `xml:"id"`
	InputPort         int    `xml:"inputPort"`
	VideoInputEnabled bool   `xml:"videoInputEnabled"`
	Name              string `xml:"name"`
	VideoFormat       string `xml:"videoFormat"`
	ResDesc           string `xml:"resDesc"`
}

type xmlStreamingChannel struct {
	XMLName xml.Name `xml:"StreamingChannel"`
	ID      int      `xml:"id"`
	Enabled bool     `xml:"enabled"`
	Video   struct {
		Enabled                  bool   `xml:"enabled"`
		VideoInputChannelID      int    `xml:"videoInputChannelID"`
		VideoCodecType           string `xml:"videoCodecType"`
		VideoResolutionWidth     int    `xml:"videoResolutionWidth"`
		VideoResolutionHeight    int    `xml:"videoResolutionHeight"`
		VideoQualityControlType  string `xml:"videoQualityControlType"`
		VBRUpperCap              int    `xml:"vbrUpperCap"`
		ConstantBitRate          int    `xml:"constantBitRate"`
		MaxFrameRate             int    `xml:"maxFrameRate"`
	} `xml:"Video"`
	Audio struct {
		Enabled              bool   `xml:"enabled"`
		AudioCompressionType string `xml:"audioCompressionType"`
	} `xml:"Audio"`
}

type xmlTrackDailyDistribution struct {
	XMLName xml.Name `xml:"trackDailyDistribution"`
	Days    []struct {
		ID         int    `xml:"id"`
		DayOfMonth int    `xml:"dayOfMonth"`
		Record     bool   `xml:"record"`
		RecordType string `xml:"recordType"`
	} `xml:"dayList>day"`
}

type xmlCMSearchResult struct {
	XMLName            xml.Name             `xml:"CMSearchResult"`
	ResponseStatus     bool                 `xml:"responseStatus"`
	ResponseStatusStrg string               `xml:"responseStatusStrg"`
	NumOfMatches       int                  `xml:"numOfMatches"`
	Matches            []xmlSearchMatchItem `xml:"matchList>searchMatchItem"`
}

type xmlSearchMatchItem struct {
	TrackID  int `xml:"trackID"`
	TimeSpan struct {
		StartTime string `xml:"startTime"`
		EndTime   string `xml:"endTime"`
	} `xml:"timeSpan"`
	MediaSegmentDescriptor struct {
		ContentType string `xml:"contentType"`
		CodecType   string `xml:"codecType"`
		PlaybackURI string `xml:"playbackURI"`
	} `xml:"mediaSegmentDescriptor"`
}

type xmlVideoOverlay struct {
	XMLName              xml.Name `xml:"VideoOverlay"`
	Version              string   `xml:"version,attr,omitempty"`
	Xmlns                string   `xml:"xmlns,attr,omitempty"`
	NormalizedScreenSize struct {
		Width  int `xml:"normalizedScreenWidth"`
		Height int `xml:"normalizedScreenHeight"`
	} `xml:"normalizedScreenSize"`
	Attribute struct {
		Transparent bool `xml:"transparent"`
		Flashing    bool `xml:"flashing"`
	} `xml:"attribute"`
	FontSize        int              `xml:"fontSize"`
	TextOverlayList []xmlTextOverlay `xml:"TextOverlayList>TextOverlay"`
	DateTimeOverlay struct {
		Enabled     bool   `xml:"enabled"`
		PositionX   int    `xml:"positionX"`
		PositionY   int    `xml:"positionY"`
		DateStyle   string `xml:"dateStyle"`
		TimeStyle   string `xml:"timeStyle"`
		DisplayWeek bool   `xml:"displayWeek"`
	} `xml:"DateTimeOverlay"`
	ChannelNameOverlay struct {
		Enabled   bool `xml:"enabled"`
		PositionX int  `xml:"positionX"`
		PositionY int  `xml:"positionY"`
	} `xml:"channelNameOverlay"`
}

type xmlTextOverlay struct {
	ID          int     `xml:"id"`
	Enabled     bool    `xml:"enabled"`
	PositionX   float64 `xml:"positionX"`
	PositionY   float64 `xml:"positionY"`
	DisplayText string  `xml:"displayText"`
}

type xmlMotionDetection struct {
	XMLName          xml.Name `xml:"MotionDetection"`
	Version          string   `xml:"version,attr,omitempty"`
	Xmlns            string   `xml:"xmlns,attr,omitempty"`
	Enabled          bool     `xml:"enabled"`
	EnableHighlight  bool     `xml:"enableHighlight"`
	SamplingInterval int      `xml:"samplingInterval"`
	StartTriggerTime int      `xml:"startTriggerTime"`
	EndTriggerTime   int      `xml:"endTriggerTime"`
	RegionType       string   `xml:"regionType"`
	Grid             struct {
		RowGranularity    int `xml:"rowGranularity"`
		ColumnGranularity int `xml:"columnGranularity"`
	} `xml:"Grid"`
	MotionDetectionLayout struct {
		SensitivityLevel int    `xml:"sensitivityLevel"`
		FactoryProtocol  string `xml:"FactoryProtocol,omitempty"`
		Layout           struct {
			GridMap string `xml:"gridMap"`
		} `xml:"layout"`
	} `xml:"MotionDetectionLayout"`
}

type xmlPrivacyMask struct {
	XMLName              xml.Name `xml:"PrivacyMask"`
	Version              string   `xml:"version,attr,omitempty"`
	Xmlns                string   `xml:"xmlns,attr,omitempty"`
	Enabled              bool     `xml:"enabled"`
	NormalizedScreenSize struct {
		Width  int `xml:"normalizedScreenWidth"`
		Height int `xml:"normalizedScreenHeight"`
	} `xml:"normalizedScreenSize"`
	Regions []xmlPrivacyMaskRegion `xml:"PrivacyMaskRegionList>PrivacyMaskRegion"`
}

type xmlPrivacyMaskRegion struct {
	ID              int  `xml:"id"`
	Enabled         bool `xml:"enabled"`
	CoordinatesList []struct {
		PositionX int `xml:"positionX"`
		PositionY int `xml:"positionY"`
	} `xml:"RegionCoordinatesList>RegionCoordinates"`
}
