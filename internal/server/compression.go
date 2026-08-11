package server

import (
	"bytes"
	"compress/gzip"
	"net/http"
	"strconv"
	"strings"

	"github.com/andybalholm/brotli"
)

// CompressAssets negotiates Brotli or gzip for JavaScript and CSS responses.
// The handler buffers only the selected asset response, which keeps the normal
// API and HTML paths unchanged.
func CompressAssets(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bw := &assetBuffer{header: make(http.Header)}
		next.ServeHTTP(bw, r)

		contentType := strings.ToLower(bw.header.Get("Content-Type"))
		if bw.status == http.StatusNotModified || !compressibleAsset(contentType) || bw.body.Len() == 0 {
			copyAssetResponse(w, bw, bw.body.Bytes())
			return
		}

		w.Header().Set("Vary", appendVary(w.Header().Get("Vary"), "Accept-Encoding"))
		encoding, ok := negotiatedEncoding(r.Header.Get("Accept-Encoding"))
		if !ok {
			http.Error(w, "encoding not acceptable", http.StatusNotAcceptable)
			return
		}
		if encoding == "" {
			copyAssetResponse(w, bw, bw.body.Bytes())
			return
		}

		var compressed bytes.Buffer
		switch encoding {
		case "br":
			writer := brotli.NewWriter(&compressed)
			_, _ = writer.Write(bw.body.Bytes())
			_ = writer.Close()
		case "gzip":
			writer := gzip.NewWriter(&compressed)
			_, _ = writer.Write(bw.body.Bytes())
			_ = writer.Close()
		}
		bw.header.Set("Content-Encoding", encoding)
		if etag := bw.header.Get("ETag"); etag != "" {
			bw.header.Set("ETag", `W/`+etag+`-`+encoding)
		}
		bw.header.Del("Content-Length")
		copyAssetResponse(w, bw, compressed.Bytes())
	})
}

type assetBuffer struct {
	header http.Header
	body   bytes.Buffer
	status int
}

func (b *assetBuffer) Header() http.Header { return b.header }

func (b *assetBuffer) WriteHeader(status int) {
	if b.status == 0 {
		b.status = status
	}
}

func (b *assetBuffer) Write(p []byte) (int, error) {
	if b.status == 0 {
		b.status = http.StatusOK
	}
	return b.body.Write(p)
}

func copyAssetResponse(w http.ResponseWriter, b *assetBuffer, body []byte) {
	for key, values := range b.header {
		w.Header()[key] = append([]string(nil), values...)
	}
	if b.status == 0 {
		b.status = http.StatusOK
	}
	w.WriteHeader(b.status)
	if b.status != http.StatusNotModified {
		_, _ = w.Write(body)
	}
}

func compressibleAsset(contentType string) bool {
	return strings.HasPrefix(contentType, "application/javascript") ||
		strings.HasPrefix(contentType, "text/javascript") ||
		strings.HasPrefix(contentType, "text/css")
}

func appendVary(existing, value string) string {
	for _, item := range strings.Split(existing, ",") {
		if strings.EqualFold(strings.TrimSpace(item), value) {
			return existing
		}
	}
	if existing == "" {
		return value
	}
	return existing + ", " + value
}

// negotiatedEncoding returns the highest quality supported encoding. Brotli
// wins ties because it normally produces smaller browser assets.
func negotiatedEncoding(header string) (string, bool) {
	if strings.TrimSpace(header) == "" {
		return "", true
	}
	q := map[string]float64{"br": -1, "gzip": -1, "identity": 1}
	wildcard := -1.0
	for _, part := range strings.Split(header, ",") {
		pieces := strings.Split(strings.TrimSpace(part), ";")
		name := strings.ToLower(strings.TrimSpace(pieces[0]))
		quality := 1.0
		for _, parameter := range pieces[1:] {
			keyValue := strings.SplitN(strings.TrimSpace(parameter), "=", 2)
			if len(keyValue) == 2 && strings.EqualFold(keyValue[0], "q") {
				parsed, err := strconv.ParseFloat(strings.TrimSpace(keyValue[1]), 64)
				if err != nil || parsed < 0 || parsed > 1 {
					quality = 0
				} else {
					quality = parsed
				}
			}
		}
		switch name {
		case "*":
			wildcard = quality
		case "br", "gzip", "identity":
			q[name] = quality
		}
	}
	for _, name := range []string{"br", "gzip", "identity"} {
		if q[name] < 0 && name != "identity" && wildcard >= 0 {
			q[name] = wildcard
		}
	}
	if q["br"] >= q["gzip"] && q["br"] > 0 {
		return "br", true
	}
	if q["gzip"] > 0 {
		return "gzip", true
	}
	if q["identity"] > 0 {
		return "", true
	}
	return "", false
}
