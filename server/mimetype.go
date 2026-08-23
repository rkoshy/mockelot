package server

import (
	"mime"
	"net/http"
	"path/filepath"
	"strings"
)

// detectMimeType returns the Content-Type for a file being served.
//
// Detection order (first non-empty result wins):
//  1. generatedMimeTypes[ext]  — mime-db derived map (1200+ entries, platform-independent)
//  2. mime.TypeByExtension(ext) — OS MIME database fallback
//  3. http.DetectContentType   — WHATWG magic-byte sniff on first 512 bytes
//  4. "application/octet-stream" — safe final fallback
//
// The generated map is produced by tools/gen_mimetypes/main.go from the
// jshttp/mime-db database (IANA > Apache > nginx priority), so results are
// identical across Linux, macOS, Windows, and containers regardless of what
// the host OS has in /etc/mime.types.
func detectMimeType(filename string, body []byte) string {
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(filename), "."))

	if ext != "" {
		// 1. Generated mime-db map — authoritative, platform-independent
		if ct, ok := generatedMimeTypes[ext]; ok {
			return ct
		}

		// 2. OS MIME database — catches any local additions
		if ct := mime.TypeByExtension("." + ext); ct != "" {
			return ct
		}
	}

	// 3. Magic-byte sniff — useful for binary types with no/wrong extension
	sniff := body
	if len(sniff) > 512 {
		sniff = sniff[:512]
	}
	ct := http.DetectContentType(sniff)

	// 4. DetectContentType returns "application/octet-stream" when it cannot
	// determine the type, which is the correct safe fallback.
	return ct
}
