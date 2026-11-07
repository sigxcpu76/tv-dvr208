package domain_test

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/sigxcpu/tv-dvr208/backend/internal/domain"
)

func TestTrackIDAndValidation(t *testing.T) {
	id, err := domain.TrackID(1, domain.StreamTypeMain)
	if err != nil || id != 101 {
		t.Fatalf("expected track 101, got %d (err=%v)", id, err)
	}
	id, err = domain.TrackID(8, domain.StreamTypeSub)
	if err != nil || id != 802 {
		t.Fatalf("expected track 802, got %d (err=%v)", id, err)
	}

	if _, err := domain.TrackID(0, domain.StreamTypeMain); !errors.Is(err, domain.ErrInvalidChannel) {
		t.Fatalf("expected ErrInvalidChannel for ch 0, got %v", err)
	}
	if _, err := domain.TrackID(9, domain.StreamTypeMain); !errors.Is(err, domain.ErrInvalidChannel) {
		t.Fatalf("expected ErrInvalidChannel for ch 9, got %v", err)
	}
	if _, err := domain.TrackID(1, 3); !errors.Is(err, domain.ErrInvalidStreamType) {
		t.Fatalf("expected ErrInvalidStreamType for stream 3, got %v", err)
	}
}

func TestMotionGridHexRoundTrip(t *testing.T) {
	// Full-screen gridMap from live TV-DVR208 ("ffffff" * 18)
	liveAllOnes := strings.Repeat("ffffff", domain.MotionRows)
	grid, err := domain.ParseMotionGridHex(liveAllOnes)
	if err != nil {
		t.Fatalf("unexpected error parsing live gridMap: %v", err)
	}
	for r := 0; r < domain.MotionRows; r++ {
		for c := 0; c < domain.MotionCols; c++ {
			if !grid[r][c] {
				t.Fatalf("expected cell [%d][%d] to be true", r, c)
			}
		}
	}

	// Encoding 22 columns of true yields "fffffc" per row (top 22 bits set in 24-bit row word)
	expectedEncoded := strings.Repeat("fffffc", domain.MotionRows)
	if got := grid.ToHex(); got != expectedEncoded {
		t.Fatalf("expected %s, got %s", expectedEncoded, got)
	}

	// Toggle specific cells and round-trip
	var custom domain.MotionGrid
	custom[0][0] = true   // bit 23 -> 0x800000
	custom[0][21] = true  // bit 2  -> 0x000004
	custom[17][10] = true // row 17, col 10 -> bit 13 -> 0x002000
	hexStr := custom.ToHex()
	if !strings.HasPrefix(hexStr, "800004") {
		t.Fatalf("expected row 0 hex 800004, got %s", hexStr[:6])
	}
	parsed, err := domain.ParseMotionGridHex(hexStr)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if parsed != custom {
		t.Fatalf("round-trip grid mismatch")
	}

	// Invalid length
	if _, err := domain.ParseMotionGridHex("ffff"); !errors.Is(err, domain.ErrInvalidMotionGrid) {
		t.Fatalf("expected ErrInvalidMotionGrid, got %v", err)
	}
}

func TestStripIMKHHeader(t *testing.T) {
	imkh := make([]byte, domain.IMKHHeaderSize)
	copy(imkh, []byte("IMKH\x01\x01\x00\x00"))
	mpegPS := []byte{0x00, 0x00, 0x01, 0xba, 0x44, 0x00, 0x04, 0x00}
	stream := append(append([]byte{}, imkh...), mpegPS...)

	r, hdr, err := domain.StripIMKHHeader(bytes.NewReader(stream))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(hdr) != domain.IMKHHeaderSize || string(hdr[:4]) != "IMKH" {
		t.Fatalf("expected 40-byte IMKH header, got %v", hdr)
	}
	rest, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read error: %v", err)
	}
	if !bytes.Equal(rest, mpegPS) {
		t.Fatalf("expected MPEG-PS payload %x, got %x", mpegPS, rest)
	}

	// Non-IMKH stream should preserve all bytes
	rawMPEG := bytes.Repeat([]byte{0x00, 0x00, 0x01, 0xba, 0xaa}, 10) // 50 bytes
	r2, hdr2, err := domain.StripIMKHHeader(bytes.NewReader(rawMPEG))
	if err != nil {
		t.Fatalf("unexpected error for raw MPEG: %v", err)
	}
	if hdr2 != nil {
		t.Fatalf("expected nil header for non-IMKH stream")
	}
	rest2, _ := io.ReadAll(r2)
	if !bytes.Equal(rest2, rawMPEG) {
		t.Fatalf("expected full raw MPEG stream preserved")
	}

	// Short stream error
	if _, _, err := domain.StripIMKHHeader(bytes.NewReader([]byte("IMKHshort"))); !errors.Is(err, domain.ErrShortIMKHStream) {
		t.Fatalf("expected ErrShortIMKHStream, got %v", err)
	}
}

func TestValidationRules(t *testing.T) {
	now := time.Now()
	q := domain.RecordingSearchQuery{
		ChannelID: 1,
		StartTime: now,
		EndTime:   now.Add(-time.Hour),
	}
	if !errors.Is(q.Validate(), domain.ErrInvalidTimeRange) {
		t.Fatalf("expected ErrInvalidTimeRange")
	}

	ptz := domain.PTZCommand{Pan: 150, Tilt: 0, Zoom: 0}
	if !errors.Is(ptz.Validate(), domain.ErrInvalidPTZSpeed) {
		t.Fatalf("expected ErrInvalidPTZSpeed")
	}

	ov := domain.VideoOverlayConfig{
		DateTime: domain.DateTimeOverlay{PositionX: 800, PositionY: 100},
	}
	if !errors.Is(ov.Validate(), domain.ErrInvalidOverlayCoord) {
		t.Fatalf("expected ErrInvalidOverlayCoord")
	}
}
