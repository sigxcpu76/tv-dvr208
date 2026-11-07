package domain

import "errors"

var (
	ErrInvalidChannel      = errors.New("invalid channel ID (must be between 1 and 8)")
	ErrInvalidStreamType   = errors.New("invalid stream type (must be 1 for main or 2 for sub)")
	ErrInvalidTimeRange    = errors.New("invalid time range (start time must be before end time)")
	ErrInvalidMotionGrid   = errors.New("invalid motion detection grid map")
	ErrInvalidPTZSpeed     = errors.New("invalid PTZ speed (must be between -100 and 100)")
	ErrInvalidOverlayCoord = errors.New("invalid overlay coordinate (must be within 704x576 normalized screen)")
	ErrShortIMKHStream     = errors.New("stream shorter than 40-byte IMKH header")
	ErrAuthentication      = errors.New("DVR authentication failed")
	ErrUpstreamFailure     = errors.New("DVR upstream request failed")
)
