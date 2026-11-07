package isapi_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sigxcpu/tv-dvr208/backend/internal/adapters/outbound/isapi"
)

func TestISAPIClientDigestAuthAndXMLParsing(t *testing.T) {
	var challengeCount int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Digest ") {
			atomic.AddInt32(&challengeCount, 1)
			w.Header().Set("WWW-Authenticate", `Digest qop="auth", realm="DVRNVRDVS", nonce="4d6b4a354e6a59304e7a6b364e44686d4e3259324f413d3d", stale="FALSE"`)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if !strings.Contains(auth, `username="operator"`) || !strings.Contains(auth, `realm="DVRNVRDVS"`) {
			w.WriteHeader(http.StatusForbidden)
			return
		}

		switch r.URL.Path {
		case "/ISAPI/System/deviceInfo":
			w.Write([]byte(`<?xml version="1.0" encoding="UTF-8" ?>
<DeviceInfo version="1.0" xmlns="http://www.std-cgi.com/ver20/XMLSchema">
<deviceName>TV-DVR208</deviceName>
<deviceID>48383633-3732-3836-3831-3c8cf8a1c032</deviceID>
<model>TV-DVR208</model>
<serialNumber>TV-DVR2080820170916CCWR863728681WCVU</serialNumber>
<macAddress>3c:8c:f8:a1:c0:32</macAddress>
<firmwareVersion>V3.4.7</firmwareVersion>
<firmwareReleasedDate>build 170123</firmwareReleasedDate>
<encoderVersion>V5.0</encoderVersion>
<encoderReleasedDate>build 160912</encoderReleasedDate>
<deviceType>IPC</deviceType>
<telecontrolID>255</telecontrolID>
</DeviceInfo>`))

		case "/ISAPI/System/Video/inputs/channels":
			w.Write([]byte(`<?xml version="1.0" encoding="UTF-8" ?>
<VideoInputChannelList version="1.0" xmlns="http://www.std-cgi.com/ver20/XMLSchema">
<VideoInputChannel version="1.0">
<id>1</id><inputPort>1</inputPort><videoInputEnabled>true</videoInputEnabled><name>Terasa</name><videoFormat>PAL</videoFormat><resDesc>1080P25</resDesc>
</VideoInputChannel>
</VideoInputChannelList>`))

		case "/ISAPI/ContentMgmt/record/tracks/101/dailyDistribution":
			body, _ := io.ReadAll(r.Body)
			if !strings.Contains(string(body), "<monthOfYear>11</monthOfYear>") {
				t.Errorf("unexpected dailyDistribution request body: %s", string(body))
			}
			w.Write([]byte(`<?xml version="1.0" encoding="UTF-8" ?>
<trackDailyDistribution version="1.0">
<dayList><day><id>7</id><dayOfMonth>7</dayOfMonth><record>true</record><recordType>time</recordType></day></dayList>
</trackDailyDistribution>`))

		case "/ISAPI/ContentMgmt/search":
			w.Write([]byte(`<?xml version="1.0" encoding="UTF-8" ?>
<CMSearchResult version="1.0">
<responseStatus>true</responseStatus>
<responseStatusStrg>OK</responseStatusStrg>
<numOfMatches>1</numOfMatches>
<matchList>
<searchMatchItem>
<trackID>101</trackID>
<timeSpan><startTime>2026-11-07T12:00:00Z</startTime><endTime>2026-11-07T12:13:10Z</endTime></timeSpan>
<mediaSegmentDescriptor>
<contentType>video</contentType>
<codecType>H.264-BP</codecType>
<playbackURI>rtsp://192.168.27.60/Streaming/tracks/101/?starttime=20261107T120000Z&amp;endtime=20261107T121310Z&amp;name=00000000127000000&amp;size=262040376</playbackURI>
</mediaSegmentDescriptor>
</searchMatchItem>
</matchList>
</CMSearchResult>`))

		case "/ISAPI/System/Video/inputs/channels/1/motionDetection":
			if r.Method == http.MethodPut {
				body, _ := io.ReadAll(r.Body)
				if !strings.Contains(string(body), "<gridMap>") {
					t.Errorf("expected gridMap in PUT motionDetection: %s", string(body))
				}
				w.WriteHeader(http.StatusOK)
				return
			}
			w.Write([]byte(`<?xml version="1.0" encoding="UTF-8" ?>
<MotionDetection version="1.0">
<enabled>true</enabled>
<enableHighlight>false</enableHighlight>
<samplingInterval>5</samplingInterval>
<startTriggerTime>1000</startTriggerTime>
<endTriggerTime>1000</endTriggerTime>
<regionType>grid</regionType>
<Grid><rowGranularity>18</rowGranularity><columnGranularity>22</columnGranularity></Grid>
<MotionDetectionLayout version="1.0">
<sensitivityLevel>3</sensitivityLevel>
<FactoryProtocol>HIKVISION</FactoryProtocol>
<layout><gridMap>` + strings.Repeat("fffffc", 18) + `</gridMap></layout>
</MotionDetectionLayout>
</MotionDetection>`))

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	client := isapi.NewClient(srv.URL, "operator", "test-secret", 5*time.Second)
	ctx := context.Background()

	// 1st request triggers 401 Digest challenge then succeeds
	info, err := client.GetDeviceInfo(ctx)
	if err != nil {
		t.Fatalf("GetDeviceInfo failed: %v", err)
	}
	if info.Model != "TV-DVR208" || info.FirmwareVersion != "V3.4.7" {
		t.Fatalf("unexpected device info: %+v", info)
	}

	// 2nd & 3rd requests reuse cached Digest challenge without additional 401 round-trips
	chs, err := client.ListVideoChannels(ctx)
	if err != nil || len(chs) != 1 || chs[0].Name != "Terasa" {
		t.Fatalf("ListVideoChannels failed: %v (%+v)", err, chs)
	}

	days, err := client.GetDailyDistribution(ctx, 101, 2026, 11)
	if err != nil || len(days) != 1 || !days[0].HasRecords {
		t.Fatalf("GetDailyDistribution failed: %v (%+v)", err, days)
	}

	segs, err := client.SearchRecordings(ctx, 101, time.Date(2026, 11, 7, 0, 0, 0, 0, time.UTC), time.Date(2026, 11, 7, 23, 59, 59, 0, time.UTC))
	if err != nil || len(segs) != 1 || segs[0].SizeBytes != 262040376 || segs[0].FileName != "00000000127000000" {
		t.Fatalf("SearchRecordings failed: %v (%+v)", err, segs)
	}

	md, err := client.GetMotionDetection(ctx, 1)
	if err != nil || !md.Enabled || md.SensitivityLevel != 3 || !md.Grid[0][0] {
		t.Fatalf("GetMotionDetection failed: %v (%+v)", err, md)
	}

	if err := client.UpdateMotionDetection(ctx, 1, *md); err != nil {
		t.Fatalf("UpdateMotionDetection failed: %v", err)
	}

	if got := atomic.LoadInt32(&challengeCount); got != 1 {
		t.Fatalf("expected exactly 1 Digest 401 challenge due to nonce caching, got %d", got)
	}
}
