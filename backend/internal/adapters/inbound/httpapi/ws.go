package httpapi

import (
	"context"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/sigxcpu/tv-dvr208/backend/internal/domain"
)

const wsGUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"

// handleLiveWS streams live fMP4 fragments over a binary WebSocket (RFC 6455) so that
// desktop browsers can stream all 8 camera channels simultaneously via MediaSource Extensions (MSE)
// without occupying the browser's 6-connection HTTP/1.1 socket pool.
func (h *Handler) handleLiveWS(w http.ResponseWriter, r *http.Request) {
	channelID, err := parseIDParam(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	streamType := parseIntQuery(r, "stream", domain.StreamTypeSub)
	if err := domain.ValidateChannelID(channelID); err != nil {
		writeDomainError(w, err)
		return
	}
	if err := domain.ValidateStreamType(streamType); err != nil {
		writeDomainError(w, err)
		return
	}

	wsKey := strings.TrimSpace(r.Header.Get("Sec-WebSocket-Key"))
	if wsKey == "" || !strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
		writeError(w, http.StatusBadRequest, "websocket upgrade required")
		return
	}

	hj, ok := w.(http.Hijacker)
	if !ok {
		writeError(w, http.StatusInternalServerError, "websocket hijack unsupported")
		return
	}
	conn, rw, err := hj.Hijack()
	if err != nil {
		return
	}
	defer conn.Close()

	sum := sha1.Sum([]byte(wsKey + wsGUID))
	acceptKey := base64.StdEncoding.EncodeToString(sum[:])

	handshake := "HTTP/1.1 101 Switching Protocols\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Accept: " + acceptKey + "\r\n\r\n"
	if _, err := rw.WriteString(handshake); err != nil {
		return
	}
	if err := rw.Flush(); err != nil {
		return
	}

	ctx, streamID := h.registerLiveStream(r.Context(), channelID)
	defer h.unregisterLiveStream(streamID)

	wsCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Monitor client disconnect or incoming close frame (opcode 0x8)
	go func() {
		defer cancel()
		buf := make([]byte, 512)
		for {
			if _, err := conn.Read(buf); err != nil {
				return
			}
		}
	}()

	writer := &wsBinaryWriter{conn: conn}
	_ = h.useCase.StreamLiveVideo(wsCtx, channelID, streamType, writer)
	_ = writer.writeClose()
}

type wsBinaryWriter struct {
	mu   sync.Mutex
	conn net.Conn
}

func (w *wsBinaryWriter) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	w.mu.Lock()
	defer w.mu.Unlock()

	_ = w.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))

	var hdr [10]byte
	hdr[0] = 0x82 // FIN=1, Opcode=0x2 (Binary frame)
	var hdrLen int
	n := len(p)
	switch {
	case n < 126:
		hdr[1] = byte(n)
		hdrLen = 2
	case n <= 65535:
		hdr[1] = 126
		binary.BigEndian.PutUint16(hdr[2:4], uint16(n))
		hdrLen = 4
	default:
		hdr[1] = 127
		binary.BigEndian.PutUint64(hdr[2:10], uint64(n))
		hdrLen = 10
	}

	if _, err := writeFull(w.conn, hdr[:hdrLen]); err != nil {
		return 0, err
	}
	if _, err := writeFull(w.conn, p); err != nil {
		return 0, err
	}
	return n, nil
}

func (w *wsBinaryWriter) writeClose() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	_ = w.conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
	_, err := writeFull(w.conn, []byte{0x88, 0x00})
	return err
}

func writeFull(w io.Writer, buf []byte) (int, error) {
	total := 0
	for total < len(buf) {
		n, err := w.Write(buf[total:])
		total += n
		if err != nil {
			return total, err
		}
	}
	return total, nil
}
