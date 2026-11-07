package isapi

import (
	"bytes"
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
)

// DigestTransport is an http.RoundTripper implementing RFC 2617 HTTP Digest Authentication.
type DigestTransport struct {
	Username  string
	Password  string
	Transport http.RoundTripper

	mu        sync.RWMutex
	challenge *digestChallenge
	nc        uint64
}

type digestChallenge struct {
	Realm     string
	Nonce     string
	Opaque    string
	Algorithm string
	Qop       string
}

func (t *DigestTransport) baseTransport() http.RoundTripper {
	if t.Transport != nil {
		return t.Transport
	}
	return http.DefaultTransport
}

func (t *DigestTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Buffer request body if present so we can replay it on a 401 Digest challenge
	var bodyBytes []byte
	if req.Body != nil {
		var err error
		bodyBytes, err = io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		req.Body.Close()
	}

	cloneReq := func() *http.Request {
		r := req.Clone(req.Context())
		if bodyBytes != nil {
			r.Body = io.NopCloser(bytes.NewReader(bodyBytes))
			r.ContentLength = int64(len(bodyBytes))
		}
		return r
	}

	// If we have a cached challenge, attach Authorization header optimistically
	t.mu.RLock()
	ch := t.challenge
	t.mu.RUnlock()

	firstReq := cloneReq()
	if ch != nil {
		authHdr, err := t.authorize(firstReq, ch)
		if err == nil {
			firstReq.Header.Set("Authorization", authHdr)
		}
	}

	resp, err := t.baseTransport().RoundTrip(firstReq)
	if err != nil || resp.StatusCode != http.StatusUnauthorized {
		return resp, err
	}

	// Parse WWW-Authenticate header from 401 response
	wwwAuth := resp.Header.Get("WWW-Authenticate")
	if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(wwwAuth)), "digest ") {
		return resp, nil
	}

	// Drain and close initial 401 response body before retrying
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	newCh := parseDigestChallenge(wwwAuth)
	t.mu.Lock()
	t.challenge = newCh
	t.mu.Unlock()

	retryReq := cloneReq()
	authHdr, err := t.authorize(retryReq, newCh)
	if err != nil {
		return nil, err
	}
	retryReq.Header.Set("Authorization", authHdr)
	return t.baseTransport().RoundTrip(retryReq)
}

func (t *DigestTransport) authorize(req *http.Request, ch *digestChallenge) (string, error) {
	uri := req.URL.RequestURI()
	ha1 := md5Hex(fmt.Sprintf("%s:%s:%s", t.Username, ch.Realm, t.Password))
	ha2 := md5Hex(fmt.Sprintf("%s:%s", req.Method, uri))

	ncVal := atomic.AddUint64(&t.nc, 1)
	ncHex := fmt.Sprintf("%08x", ncVal)
	cnonce := randomHex(8)

	var responseHash string
	qopUsed := ""
	if strings.Contains(ch.Qop, "auth") {
		qopUsed = "auth"
		responseHash = md5Hex(fmt.Sprintf("%s:%s:%s:%s:%s:%s", ha1, ch.Nonce, ncHex, cnonce, qopUsed, ha2))
	} else {
		responseHash = md5Hex(fmt.Sprintf("%s:%s:%s", ha1, ch.Nonce, ha2))
	}

	var parts []string
	parts = append(parts, fmt.Sprintf(`username="%s"`, t.Username))
	parts = append(parts, fmt.Sprintf(`realm="%s"`, ch.Realm))
	parts = append(parts, fmt.Sprintf(`nonce="%s"`, ch.Nonce))
	parts = append(parts, fmt.Sprintf(`uri="%s"`, uri))
	parts = append(parts, fmt.Sprintf(`response="%s"`, responseHash))
	if ch.Opaque != "" {
		parts = append(parts, fmt.Sprintf(`opaque="%s"`, ch.Opaque))
	}
	if ch.Algorithm != "" {
		parts = append(parts, fmt.Sprintf(`algorithm=%s`, ch.Algorithm))
	}
	if qopUsed != "" {
		parts = append(parts, fmt.Sprintf(`qop=%s`, qopUsed))
		parts = append(parts, fmt.Sprintf(`nc=%s`, ncHex))
		parts = append(parts, fmt.Sprintf(`cnonce="%s"`, cnonce))
	}

	return "Digest " + strings.Join(parts, ", "), nil
}

func parseDigestChallenge(header string) *digestChallenge {
	ch := &digestChallenge{}
	raw := strings.TrimSpace(header)
	if len(raw) >= 7 && strings.EqualFold(raw[:7], "digest ") {
		raw = strings.TrimSpace(raw[7:])
	}

	// Split key=value pairs respecting quotes
	var pairs []string
	var cur strings.Builder
	inQuotes := false
	for i := 0; i < len(raw); i++ {
		c := raw[i]
		if c == '"' {
			inQuotes = !inQuotes
			cur.WriteByte(c)
		} else if c == ',' && !inQuotes {
			pairs = append(pairs, strings.TrimSpace(cur.String()))
			cur.Reset()
		} else {
			cur.WriteByte(c)
		}
	}
	if cur.Len() > 0 {
		pairs = append(pairs, strings.TrimSpace(cur.String()))
	}

	for _, p := range pairs {
		kv := strings.SplitN(p, "=", 2)
		if len(kv) != 2 {
			continue
		}
		k := strings.ToLower(strings.TrimSpace(kv[0]))
		v := strings.Trim(strings.TrimSpace(kv[1]), `"`)
		switch k {
		case "realm":
			ch.Realm = v
		case "nonce":
			ch.Nonce = v
		case "opaque":
			ch.Opaque = v
		case "algorithm":
			ch.Algorithm = v
		case "qop":
			ch.Qop = v
		}
	}
	return ch
}

func md5Hex(s string) string {
	sum := md5.Sum([]byte(s))
	return hex.EncodeToString(sum[:])
}

func randomHex(numBytes int) string {
	b := make([]byte, numBytes)
	if _, err := rand.Read(b); err != nil {
		return "0123456789abcdef"
	}
	return hex.EncodeToString(b)
}
