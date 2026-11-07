package domain

import (
	"bytes"
	"io"
)

// IMKHHeaderSize is the exact 40-byte (0x28) Hikvision system header size discovered in NetStream.dll (FUN_1000f4a0).
const IMKHHeaderSize = 40

// IMKHMagic is the 4-byte ASCII signature ("IMKH") at offset 0 of Hikvision media streams.
var IMKHMagic = []byte{'I', 'M', 'K', 'H'}

// StripIMKHHeader inspects the first 40 bytes of r. If the stream begins with the 40-byte "IMKH"
// Hikvision header, those 40 bytes are consumed and returned separately so the returned io.Reader
// begins directly at the MPEG-2 Program Stream (0x000001BA). If it does not start with "IMKH",
// the bytes are prepended back into the returned io.Reader seamlessly.
func StripIMKHHeader(r io.Reader) (io.Reader, []byte, error) {
	header := make([]byte, IMKHHeaderSize)
	n, err := io.ReadFull(r, header)
	if err != nil {
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			return bytes.NewReader(header[:n]), nil, ErrShortIMKHStream
		}
		return nil, nil, err
	}

	if bytes.Equal(header[:4], IMKHMagic) {
		return r, header, nil
	}

	// Not an IMKH-prefixed stream; replay all 40 bytes untouched.
	return io.MultiReader(bytes.NewReader(header), r), nil, nil
}
