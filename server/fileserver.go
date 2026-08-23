package server

import (
	"bytes"
	"fmt"
	"log"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"mockelot/models"
)

// ssiResponseCapture is a minimal http.ResponseWriter that buffers status, headers,
// and body in memory for SSI sub-requests — the same role as httptest.ResponseRecorder
// but with no dependency on the net/http/httptest package.
type ssiResponseCapture struct {
	code   int
	header http.Header
	body   bytes.Buffer
}

func newSSICapture() *ssiResponseCapture {
	return &ssiResponseCapture{code: http.StatusOK, header: make(http.Header)}
}

func (rc *ssiResponseCapture) Header() http.Header        { return rc.header }
func (rc *ssiResponseCapture) WriteHeader(code int)        { rc.code = code }
func (rc *ssiResponseCapture) Write(b []byte) (int, error) { return rc.body.Write(b) }

// ssiIncludeRE matches Apache-style SSI virtual include directives:
//
//	<!--#include virtual="/some/path" -->
//	<!--#include virtual="/some/path"-->
//
// Group 1 captures the virtual path value (may be single- or double-quoted).
var ssiIncludeRE = regexp.MustCompile(`<!--#include\s+virtual=["']([^"']+)["']\s*-->`)

// maxSSIDepth caps recursive SSI include depth to prevent cycles.
const maxSSIDepth = 10

// FileServerHandler serves files from a local directory, with optional SSI
// processing. SSI virtual includes are resolved by re-issuing an internal
// sub-request through the full ResponseHandler pipeline — exactly the same
// path that a browser request would take. This means endpoint matching,
// prefix/translation, and header manipulation all apply to included fragments.
type FileServerHandler struct {
	proxyHandler *ProxyHandler // reused for header manipulation, status translation, logging
}

// NewFileServerHandler creates a FileServerHandler. The proxyHandler reference
// is used for header manipulation and logging helpers only; no backend HTTP
// requests are made.
func NewFileServerHandler(proxyHandler *ProxyHandler) *FileServerHandler {
	return &FileServerHandler{proxyHandler: proxyHandler}
}

// ServeHTTP handles a file server request. translatedPath is the path produced
// by the endpoint's translation pipeline (e.g. "/iris1/pages/queues.shtml").
// It is joined onto cfg.BasePath to locate the file on disk.
func (f *FileServerHandler) ServeHTTP(
	w http.ResponseWriter,
	r *http.Request,
	endpoint *models.Endpoint,
	translatedPath string,
	responseHandler *ResponseHandler, // needed to issue SSI sub-requests
) {
	cfg := endpoint.FileServerConfig
	if cfg == nil {
		http.Error(w, "File server configuration missing", http.StatusInternalServerError)
		return
	}

	startTime := time.Now()
	requestID := uuid.New().String()

	// --- Resolve disk path ------------------------------------------------
	// translatedPath already has the URL prefix stripped/translated by the
	// endpoint machinery. Strip any leading slash and join with BasePath.
	basePath := expandHomedir(cfg.BasePath)
	rel := strings.TrimPrefix(translatedPath, "/")
	diskPath := filepath.Join(basePath, filepath.FromSlash(rel))

	// Security: ensure the resolved path stays inside BasePath.
	cleanBase := filepath.Clean(basePath)
	cleanDisk := filepath.Clean(diskPath)
	if !strings.HasPrefix(cleanDisk, cleanBase+string(filepath.Separator)) && cleanDisk != cleanBase {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// --- Read file --------------------------------------------------------
	fileBytes, err := os.ReadFile(cleanDisk)
	if err != nil {
		if os.IsNotExist(err) {
			// File not found on disk — fall back to the overlay (real server).
			// This handles files that exist on the real server but not locally
			// (e.g. branding assets, generated resources, etc.).
			log.Printf("[FileServer] %s not found locally, falling back to overlay", cleanDisk)
			requestDomain := extractDomain(r)
			responseHandler.configMutex.RLock()
			domainTakeover := responseHandler.config.DomainTakeover
			responseHandler.configMutex.RUnlock()
			if responseHandler.overlayHandler != nil &&
				responseHandler.overlayHandler.shouldUseOverlay(requestDomain, domainTakeover) {
				if err := responseHandler.overlayHandler.handleOverlay(w, r, requestDomain); err != nil {
					log.Printf("[FileServer] Overlay fallback failed for %s: %v", r.URL.Path, err)
					http.Error(w, "Not Found", http.StatusNotFound)
				}
			} else {
				http.Error(w, "Not Found", http.StatusNotFound)
			}
		} else {
			f.logFileRequest(requestID, endpoint, r, translatedPath, cleanDisk, http.StatusInternalServerError, 0, startTime)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
		return
	}

	// --- SSI processing ---------------------------------------------------
	body := fileBytes
	if cfg.EnableSSI && isSSICandidate(cleanDisk) {
		body = f.processSSI(fileBytes, r, responseHandler, 0)
	}

	// --- Content-Type -----------------------------------------------------
	ct := mime.TypeByExtension(filepath.Ext(cleanDisk))
	if ct == "" {
		ct = http.DetectContentType(body)
	}
	// .shtml should be served as text/html regardless of the SSI processing.
	if strings.HasSuffix(strings.ToLower(cleanDisk), ".shtml") {
		ct = "text/html; charset=utf-8"
	}

	// --- Apply outbound header manipulation from ProxyConfig --------------
	w.Header().Set("Content-Type", ct)
	if cfg.ProxyConfig != nil && f.proxyHandler != nil {
		f.proxyHandler.applyHeaderManipulation(w.Header(), cfg.ProxyConfig.OutboundHeaders, r)
	}

	// --- Status code translation ------------------------------------------
	statusCode := http.StatusOK
	if cfg.ProxyConfig != nil && !cfg.ProxyConfig.StatusPassthrough && f.proxyHandler != nil {
		statusCode = f.proxyHandler.translateStatusCode(statusCode, cfg.ProxyConfig.StatusTranslation)
	}

	// --- Write response ---------------------------------------------------
	w.WriteHeader(statusCode)
	w.Write(body)

	f.logFileRequest(requestID, endpoint, r, translatedPath, cleanDisk, statusCode, len(body), startTime)
}

// processSSI scans content for SSI virtual include directives and replaces
// each with the output of an internal sub-request through the ResponseHandler
// pipeline. depth prevents infinite recursion.
func (f *FileServerHandler) processSSI(
	content []byte,
	originalReq *http.Request,
	responseHandler *ResponseHandler,
	depth int,
) []byte {
	if depth >= maxSSIDepth {
		log.Printf("[SSI] Max include depth (%d) reached, stopping recursion", maxSSIDepth)
		return content
	}

	return ssiIncludeRE.ReplaceAllFunc(content, func(match []byte) []byte {
		submatches := ssiIncludeRE.FindSubmatch(match)
		if len(submatches) < 2 {
			return match
		}
		virtualPath := string(submatches[1])

		// Build a synthetic GET request for the virtual path, inheriting
		// the original request's host/headers so domain filters still work.
		subReq, err := http.NewRequestWithContext(
			originalReq.Context(),
			http.MethodGet,
			virtualPath,
			http.NoBody,
		)
		if subReq != nil {
			subReq.Body = http.NoBody
		}
		if err != nil {
			log.Printf("[SSI] Failed to build sub-request for %q: %v", virtualPath, err)
			return []byte{} // Apache default: output nothing for broken includes
		}

		// Copy host and key headers from the original request so the
		// endpoint domain filter and header expressions still see the
		// real browser context.
		subReq.Host = originalReq.Host
		for _, hdr := range []string{"Cookie", "Accept", "Accept-Language", "User-Agent"} {
			if v := originalReq.Header.Get(hdr); v != "" {
				subReq.Header.Set(hdr, v)
			}
		}
		// Mark as internal SSI sub-request so we can avoid logging it.
		subReq.Header.Set("X-SSI-Depth", fmt.Sprintf("%d", depth+1))

		rec := newSSICapture()
		responseHandler.HandleRequest(rec, subReq)

		result := rec.body.Bytes()
		if rec.code == http.StatusNotFound || rec.code == http.StatusForbidden {
			log.Printf("[SSI] Include %q returned %d, substituting empty content", virtualPath, rec.code)
			return []byte{}
		}

		// The sub-request already ran SSI processing if the included file
		// matched an SSI-enabled file server endpoint — no need to recurse
		// again here. But if the response came back as raw bytes (e.g. from
		// a non-file-server endpoint), we don't re-process.
		return result
	})
}

// isSSICandidate returns true for file extensions that should have SSI
// directives processed. Matches Apache's default behaviour (.shtml) plus
// plain .html for flexibility.
func isSSICandidate(diskPath string) bool {
	ext := strings.ToLower(filepath.Ext(diskPath))
	return ext == ".shtml" || ext == ".html" || ext == ".htm"
}

// expandHomedir replaces a leading ~ with the current user's home directory.
func expandHomedir(path string) string {
	if !strings.HasPrefix(path, "~") {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	return filepath.Join(home, path[1:])
}

// logFileRequest records the request in the traffic log.
func (f *FileServerHandler) logFileRequest(
	requestID string,
	endpoint *models.Endpoint,
	r *http.Request,
	translatedPath string,
	diskPath string,
	statusCode int,
	bodyLen int,
	startTime time.Time,
) {
	if f.proxyHandler == nil || f.proxyHandler.logger == nil {
		return
	}

	// Skip logging for internal SSI sub-requests to keep the traffic log clean.
	if r.Header.Get("X-SSI-Depth") != "" {
		return
	}

	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	fullURL := scheme + "://" + r.Host + r.URL.RequestURI()

	reqHeaders := make(map[string][]string, len(r.Header))
	for k, v := range r.Header {
		cp := make([]string, len(v))
		copy(cp, v)
		reqHeaders[k] = cp
	}

	queryParams := make(map[string][]string)
	for k, v := range r.URL.Query() {
		cp := make([]string, len(v))
		copy(cp, v)
		queryParams[k] = cp
	}

	rttMs := time.Since(startTime).Milliseconds()
	zeroMs := int64(0)

	requestLog := models.RequestLog{
		ID:         requestID,
		Timestamp:  time.Now().Format(time.RFC3339),
		EndpointID: endpoint.ID,
	}
	requestLog.ClientRequest.Method = r.Method
	requestLog.ClientRequest.FullURL = fullURL
	requestLog.ClientRequest.Path = r.URL.Path
	requestLog.ClientRequest.QueryParams = queryParams
	requestLog.ClientRequest.Headers = reqHeaders
	requestLog.ClientRequest.Protocol = r.Proto
	requestLog.ClientRequest.SourceIP = r.RemoteAddr
	requestLog.ClientRequest.UserAgent = r.UserAgent()

	requestLog.ClientResponse.StatusCode = &statusCode
	requestLog.ClientResponse.StatusText = http.StatusText(statusCode)
	requestLog.ClientResponse.DelayMs = &zeroMs
	requestLog.ClientResponse.RTTMs = &rttMs

	// Represent the disk path as the "backend" so the traffic log shows
	// where the file came from.
	backendURL := "file://" + diskPath
	bodyLenInt := bodyLen
	_ = bodyLenInt
	requestLog.BackendRequest = &struct {
		Method      string              `json:"method"`
		FullURL     string              `json:"full_url"`
		Path        string              `json:"path"`
		QueryParams map[string][]string `json:"query_params,omitempty"`
		Headers     map[string][]string `json:"headers,omitempty"`
		Body        string              `json:"body,omitempty"`
	}{
		Method:  r.Method,
		FullURL: backendURL,
		Path:    translatedPath,
	}
	requestLog.BackendResponse = &struct {
		StatusCode *int                `json:"status_code,omitempty"`
		StatusText string              `json:"status_text,omitempty"`
		Headers    map[string][]string `json:"headers,omitempty"`
		Body       string              `json:"body,omitempty"`
		DelayMs    *int64              `json:"delay_ms,omitempty"`
		RTTMs      *int64              `json:"rtt_ms,omitempty"`
	}{
		StatusCode: &statusCode,
		StatusText: http.StatusText(statusCode),
		DelayMs:    &zeroMs,
		RTTMs:      &rttMs,
	}

	f.proxyHandler.logger.UpdateRequestLog(requestLog)
}
