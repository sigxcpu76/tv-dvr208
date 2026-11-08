package ffmpeg

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os/exec"
	"sync"
	"time"
)

// CommandRunner abstracts exec.CommandContext for deterministic unit testing.
type CommandRunner func(ctx context.Context, name string, args ...string) *exec.Cmd

// Streamer implements ports.MediaStreamPort using zero-transcode FFmpeg remuxing.
type Streamer struct {
	dvrHost    string
	rtspPort   int
	username   string
	password   string
	ffmpegBin  string
	cmdBuilder CommandRunner
	startMu    sync.Mutex
	lastStart  time.Time
}

// NewStreamer constructs a new FFmpeg media streamer adapter.
func NewStreamer(dvrHost string, rtspPort int, username, password string) *Streamer {
	if rtspPort <= 0 {
		rtspPort = 554
	}
	return &Streamer{
		dvrHost:    dvrHost,
		rtspPort:   rtspPort,
		username:   username,
		password:   password,
		ffmpegBin:  "ffmpeg",
		cmdBuilder: exec.CommandContext,
	}
}

// WithCommandRunner overrides the command runner (used in unit tests).
func (s *Streamer) WithCommandRunner(runner CommandRunner) *Streamer {
	s.cmdBuilder = runner
	return s
}

func (s *Streamer) rtspURL(pathAndQuery string) string {
	u := &url.URL{
		Scheme: "rtsp",
		User:   url.UserPassword(s.username, s.password),
		Host:   fmt.Sprintf("%s:%d", s.dvrHost, s.rtspPort),
	}
	return u.String() + pathAndQuery
}

// LiveRTSPArgs returns the FFmpeg CLI arguments for zero-transcode live RTSP -> fragmented MP4 streaming.
func (s *Streamer) LiveRTSPArgs(trackID int) []string {
	rtspURI := s.rtspURL(fmt.Sprintf("/PSIA/streaming/channels/%d", trackID))
	return []string{
		"-hide_banner",
		"-loglevel", "error",
		"-rtsp_transport", "tcp",
		"-i", rtspURI,
		"-copyts",
		"-c:v", "copy",
		"-an",
		"-f", "mp4",
		"-movflags", "frag_keyframe+empty_moov+default_base_moof+frag_every_frame",
		"-flush_packets", "1",
		"pipe:1",
	}
}

// ArchiveRTSPArgs returns the FFmpeg CLI arguments for RTSP VOD archive playback -> fragmented MP4.
func (s *Streamer) ArchiveRTSPArgs(trackID int, startTime, endTime time.Time) []string {
	path := fmt.Sprintf("/PSIA/streaming/tracks/%d?starttime=%s&endtime=%s",
		trackID,
		startTime.UTC().Format("20060102T150405Z"),
		endTime.UTC().Format("20060102T150405Z"),
	)
	rtspURI := s.rtspURL(path)
	return []string{
		"-hide_banner",
		"-loglevel", "error",
		"-rtsp_transport", "tcp",
		"-i", rtspURI,
		"-c:v", "copy",
		"-an",
		"-f", "mp4",
		"-movflags", "frag_keyframe+empty_moov+default_base_moof",
		"pipe:1",
	}
}

// RemuxPipeArgs returns the FFmpeg CLI arguments for remuxing an IMKH-stripped MPEG-PS stream from stdin to fragmented MP4 on stdout.
func (s *Streamer) RemuxPipeArgs() []string {
	return []string{
		"-hide_banner",
		"-loglevel", "error",
		"-f", "mpeg",
		"-i", "pipe:0",
		"-c:v", "copy",
		"-an",
		"-f", "mp4",
		"-movflags", "frag_keyframe+empty_moov+default_base_moof",
		"pipe:1",
	}
}

func (s *Streamer) StreamLiveMP4(ctx context.Context, trackID int, w io.Writer) error {
	if err := s.paceRTSPStart(ctx); err != nil {
		return nil
	}
	args := s.LiveRTSPArgs(trackID)
	return s.runFFmpeg(ctx, args, nil, w)
}

func (s *Streamer) paceRTSPStart(ctx context.Context) error {
	s.startMu.Lock()
	now := time.Now()
	var wait time.Duration
	if !s.lastStart.IsZero() {
		elapsed := now.Sub(s.lastStart)
		if elapsed < 150*time.Millisecond {
			wait = 150*time.Millisecond - elapsed
		}
	}
	s.lastStart = now.Add(wait)
	s.startMu.Unlock()

	if wait > 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(wait):
		}
	}
	return nil
}

func (s *Streamer) StreamArchiveRTSPToMP4(ctx context.Context, trackID int, startTime, endTime time.Time, w io.Writer) error {
	args := s.ArchiveRTSPArgs(trackID, startTime, endTime)
	return s.runFFmpeg(ctx, args, nil, w)
}

func (s *Streamer) RemuxMPEGPSStreamToMP4(ctx context.Context, mpegPSReader io.Reader, w io.Writer) error {
	args := s.RemuxPipeArgs()
	return s.runFFmpeg(ctx, args, mpegPSReader, w)
}

func (s *Streamer) runFFmpeg(ctx context.Context, args []string, stdin io.Reader, stdout io.Writer) error {
	cmd := s.cmdBuilder(ctx, s.ffmpegBin, args...)
	cmd.WaitDelay = 2 * time.Second
	if stdin != nil {
		cmd.Stdin = stdin
	}
	cmd.Stdout = &flushingWriter{w: stdout}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err := cmd.Run()
	if ctx.Err() != nil {
		// Client disconnected or canceled stream; not an error
		return nil
	}
	if err != nil {
		return fmt.Errorf("ffmpeg failed: %w (stderr: %s)", err, stderr.String())
	}
	return nil
}

type flushingWriter struct {
	w io.Writer
}

func (fw *flushingWriter) Write(p []byte) (int, error) {
	n, err := fw.w.Write(p)
	if flusher, ok := fw.w.(http.Flusher); ok {
		flusher.Flush()
	}
	return n, err
}
