package shop

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
)

// Body is a precomputed response body with an optional gzip variant.
type Body struct {
	Raw  []byte
	Gz   []byte // nil when compression does not pay off
	ETag string
	// lazyGz marks an uncached one-off body: it is gzipped per request, at
	// BestSpeed, and only when the client accepts gzip.
	lazyGz bool
}

const gzipMinSize = 1024

// NewBody builds a Body, compressing it once if it is large enough.
func NewBody(raw []byte) *Body {
	sum := sha256.Sum256(raw)
	b := &Body{Raw: raw, ETag: `"` + base64.RawURLEncoding.EncodeToString(sum[:12]) + `"`}
	if len(raw) >= gzipMinSize {
		var buf bytes.Buffer
		zw, _ := gzip.NewWriterLevel(&buf, gzip.BestCompression)
		zw.Write(raw)
		zw.Close()
		if buf.Len() < len(raw) {
			b.Gz = buf.Bytes()
		}
	}
	return b
}

// newUncachedBody builds a Body for a response that will be served once. It
// skips the up-front BestCompression pass that NewBody pays for cached bodies.
func newUncachedBody(raw []byte) *Body {
	sum := sha256.Sum256(raw)
	return &Body{Raw: raw, ETag: `"` + base64.RawURLEncoding.EncodeToString(sum[:12]) + `"`, lazyGz: len(raw) >= gzipMinSize}
}

func acceptsGzip(r *http.Request) bool {
	for _, v := range r.Header.Values("Accept-Encoding") {
		if strings.Contains(v, "gzip") {
			return true
		}
	}
	return false
}

// serveBody writes a precomputed body with content negotiation and ETag support.
func serveBody(w http.ResponseWriter, r *http.Request, b *Body, contentType, cacheControl string) {
	h := w.Header()
	h.Set("Content-Type", contentType)
	h.Set("ETag", b.ETag)
	if cacheControl != "" {
		h.Set("Cache-Control", cacheControl)
	}
	if b.Gz != nil || b.lazyGz {
		h.Add("Vary", "Accept-Encoding")
	}
	if inm := r.Header.Get("If-None-Match"); inm != "" && strings.Contains(inm, b.ETag) {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	data := b.Raw
	if b.Gz != nil && acceptsGzip(r) {
		h.Set("Content-Encoding", "gzip")
		data = b.Gz
	} else if b.lazyGz && acceptsGzip(r) {
		var buf bytes.Buffer
		zw, _ := gzip.NewWriterLevel(&buf, gzip.BestSpeed)
		zw.Write(b.Raw)
		zw.Close()
		if buf.Len() < len(b.Raw) {
			h.Set("Content-Encoding", "gzip")
			data = buf.Bytes()
		}
	}
	h.Set("Content-Length", strconv.Itoa(len(data)))
	w.WriteHeader(http.StatusOK)
	if r.Method != http.MethodHead {
		w.Write(data)
	}
}

func writeRaw(w http.ResponseWriter, status int, data []byte) {
	h := w.Header()
	h.Set("Content-Type", "application/json")
	h.Set("Content-Length", strconv.Itoa(len(data)))
	w.WriteHeader(status)
	w.Write(data)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	data, err := json.Marshal(v)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "encoding error")
		return
	}
	writeRaw(w, status, data)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	data, _ := json.Marshal(map[string]string{"error": msg})
	writeRaw(w, status, data)
}

const maxBodyBytes = 1 << 20

// decodeBody decodes a JSON request body into v, writing a 400 on failure.
func decodeBody(w http.ResponseWriter, r *http.Request, v any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	if err := dec.Decode(v); err != nil {
		msg := "invalid JSON body"
		if errors.Is(err, io.EOF) {
			msg = "request body is required"
		}
		writeError(w, http.StatusBadRequest, msg)
		return false
	}
	return true
}
