package ffmpeg_test

import (
	"bytes"
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/sigxcpu/tv-dvr208/backend/internal/adapters/outbound/ffmpeg"
)

func TestStreamerArgsAndPiping(t *testing.T) {
	s := ffmpeg.NewStreamer("192.168.1.64", 554, "operator", "test-secret")

	liveArgs := strings.Join(s.LiveRTSPArgs(102), " ")
	if !strings.Contains(liveArgs, "/PSIA/streaming/channels/102") || !strings.Contains(liveArgs, "frag_keyframe+empty_moov+default_base_moof") {
		t.Fatalf("unexpected live args: %s", liveArgs)
	}

	st := time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC)
	et := time.Date(2026, 11, 7, 12, 13, 10, 0, time.UTC)
	vodArgs := strings.Join(s.ArchiveRTSPArgs(101, st, et), " ")
	if !strings.Contains(vodArgs, "/PSIA/streaming/tracks/101?starttime=20261107T120000Z&endtime=20261107T121310Z") {
		t.Fatalf("unexpected vod args: %s", vodArgs)
	}

	// Test RemuxMPEGPSStreamToMP4 piping using `cat` as fake command runner
	s.WithCommandRunner(func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "cat")
	})

	input := []byte{0x00, 0x00, 0x01, 0xba, 0xaa, 0xbb}
	var out bytes.Buffer
	if err := s.RemuxMPEGPSStreamToMP4(context.Background(), bytes.NewReader(input), &out); err != nil {
		t.Fatalf("RemuxMPEGPSStreamToMP4 error: %v", err)
	}
	if !bytes.Equal(out.Bytes(), input) {
		t.Fatalf("expected piped bytes %x, got %x", input, out.Bytes())
	}
}
