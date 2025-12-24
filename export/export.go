package export

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"mockelot/models"
)

type LogExporter struct {
	outputDir string
}

func NewLogExporter(outputDir string) *LogExporter {
	if outputDir == "" {
		outputDir = "exports"
	}
	return &LogExporter{outputDir: outputDir}
}

func (le *LogExporter) ExportToCSV(logs []models.RequestLog) (string, error) {
	// Ensure export directory exists
	if err := os.MkdirAll(le.outputDir, 0755); err != nil {
		return "", fmt.Errorf("could not create export directory: %v", err)
	}

	// Generate filename with timestamp
	filename := fmt.Sprintf("request_logs_%s.csv", time.Now().Format("20060102_150405"))
	fullPath := filepath.Join(le.outputDir, filename)

	file, err := os.Create(fullPath)
	if err != nil {
		return "", fmt.Errorf("could not create CSV file: %v", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write CSV headers
	headers := []string{
		"ID", "Timestamp", "Method", "Path", "SourceIP", "UserAgent", "Protocol",
	}
	if err := writer.Write(headers); err != nil {
		return "", fmt.Errorf("error writing CSV headers: %v", err)
	}

	// Write log entries
	for _, log := range logs {
		record := []string{
			log.ID,
			log.Timestamp,
			log.ClientRequest.Method,
			log.ClientRequest.Path,
			log.ClientRequest.SourceIP,
			log.ClientRequest.UserAgent,
			log.ClientRequest.Protocol,
		}
		if err := writer.Write(record); err != nil {
			return "", fmt.Errorf("error writing log entry to CSV: %v", err)
		}
	}

	return fullPath, nil
}

func (le *LogExporter) ExportToJSON(logs []models.RequestLog) (string, error) {
	// Ensure export directory exists
	if err := os.MkdirAll(le.outputDir, 0755); err != nil {
		return "", fmt.Errorf("could not create export directory: %v", err)
	}

	// Generate filename with timestamp
	filename := fmt.Sprintf("request_logs_%s.json", time.Now().Format("20060102_150405"))
	fullPath := filepath.Join(le.outputDir, filename)

	file, err := os.Create(fullPath)
	if err != nil {
		return "", fmt.Errorf("could not create JSON file: %v", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(logs); err != nil {
		return "", fmt.Errorf("error writing logs to JSON: %v", err)
	}

	return fullPath, nil
}

// HAR (HTTP Archive) format structures
type HARLog struct {
	Log HARContent `json:"log"`
}

type HARContent struct {
	Version string      `json:"version"`
	Creator HARCreator  `json:"creator"`
	Entries []HAREntry  `json:"entries"`
}

type HARCreator struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type HAREntry struct {
	StartedDateTime string      `json:"startedDateTime"`
	Time            float64     `json:"time"`
	Request         HARRequest  `json:"request"`
	Response        HARResponse `json:"response"`
}

type HARRequest struct {
	Method      string            `json:"method"`
	URL         string            `json:"url"`
	HTTPVersion string            `json:"httpVersion"`
	Headers     []HARNameValue    `json:"headers"`
	QueryString []HARNameValue    `json:"queryString"`
	PostData    *HARPostData      `json:"postData,omitempty"`
}

type HARResponse struct {
	Status      int               `json:"status"`
	StatusText  string            `json:"statusText"`
	HTTPVersion string            `json:"httpVersion"`
	Headers     []HARNameValue    `json:"headers"`
	Content     HARContent_       `json:"content"`
}

type HARContent_ struct {
	Size     int    `json:"size"`
	MimeType string `json:"mimeType"`
	Text     string `json:"text,omitempty"`
}

type HARNameValue struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type HARPostData struct {
	MimeType string `json:"mimeType"`
	Text     string `json:"text"`
}

// ExportToHAR exports logs in HAR (HTTP Archive) 1.2 format
// side can be "client" or "backend"
func (le *LogExporter) ExportToHAR(logs []models.RequestLog, side string) (string, error) {
	// Ensure export directory exists
	if err := os.MkdirAll(le.outputDir, 0755); err != nil {
		return "", fmt.Errorf("could not create export directory: %v", err)
	}

	// Generate filename with timestamp
	filename := fmt.Sprintf("request_logs_%s_%s.har", side, time.Now().Format("20060102_150405"))
	fullPath := filepath.Join(le.outputDir, filename)

	// Build HAR structure
	entries := make([]HAREntry, 0, len(logs))
	for _, log := range logs {
		// Select client or backend side
		var method, fullURL, body string
		var queryParams, reqHeaders, respHeaders map[string][]string
		var statusCode *int
		var statusText string
		var respBody string
		var rttMs *int64

		if side == "backend" && log.BackendRequest != nil {
			method = log.BackendRequest.Method
			fullURL = log.BackendRequest.FullURL
			queryParams = log.BackendRequest.QueryParams
			reqHeaders = log.BackendRequest.Headers
			body = log.BackendRequest.Body

			if log.BackendResponse != nil {
				statusCode = log.BackendResponse.StatusCode
				statusText = log.BackendResponse.StatusText
				respHeaders = log.BackendResponse.Headers
				respBody = log.BackendResponse.Body
				rttMs = log.BackendResponse.RTTMs
			}
		} else {
			method = log.ClientRequest.Method
			fullURL = log.ClientRequest.FullURL
			queryParams = log.ClientRequest.QueryParams
			reqHeaders = log.ClientRequest.Headers
			body = log.ClientRequest.Body

			statusCode = log.ClientResponse.StatusCode
			statusText = log.ClientResponse.StatusText
			respHeaders = log.ClientResponse.Headers
			respBody = log.ClientResponse.Body
			rttMs = log.ClientResponse.RTTMs
		}

		// Parse query string
		harQueryParams := []HARNameValue{}
		for key, values := range queryParams {
			for _, value := range values {
				harQueryParams = append(harQueryParams, HARNameValue{Name: key, Value: value})
			}
		}

		// Convert request headers
		harReqHeaders := make([]HARNameValue, 0, len(reqHeaders))
		for k, values := range reqHeaders {
			for _, v := range values {
				harReqHeaders = append(harReqHeaders, HARNameValue{Name: k, Value: v})
			}
		}

		// Convert response headers
		harRespHeaders := make([]HARNameValue, 0, len(respHeaders))
		for k, values := range respHeaders {
			for _, v := range values {
				harRespHeaders = append(harRespHeaders, HARNameValue{Name: k, Value: v})
			}
		}

		// Build request
		harReq := HARRequest{
			Method:      method,
			URL:         fullURL,
			HTTPVersion: "HTTP/1.1",
			Headers:     harReqHeaders,
			QueryString: harQueryParams,
		}

		// Add post data if present
		if body != "" {
			contentType := ""
			if values, ok := reqHeaders["Content-Type"]; ok && len(values) > 0 {
				contentType = values[0]
			}
			harReq.PostData = &HARPostData{
				MimeType: contentType,
				Text:     body,
			}
		}

		// Build response
		status := 0
		if statusCode != nil {
			status = *statusCode
		}

		respContentType := ""
		if values, ok := respHeaders["Content-Type"]; ok && len(values) > 0 {
			respContentType = values[0]
		}

		harResp := HARResponse{
			Status:      status,
			StatusText:  statusText,
			HTTPVersion: "HTTP/1.1",
			Headers:     harRespHeaders,
			Content: HARContent_{
				Size:     len(respBody),
				MimeType: respContentType,
				Text:     respBody,
			},
		}

		// Calculate time (RTT in ms)
		timeMs := 0.0
		if rttMs != nil {
			timeMs = float64(*rttMs)
		}

		entry := HAREntry{
			StartedDateTime: log.Timestamp,
			Time:            timeMs,
			Request:         harReq,
			Response:        harResp,
		}
		entries = append(entries, entry)
	}

	har := HARLog{
		Log: HARContent{
			Version: "1.2",
			Creator: HARCreator{
				Name:    "Mockelot",
				Version: "1.0",
			},
			Entries: entries,
		},
	}

	// Write to file
	file, err := os.Create(fullPath)
	if err != nil {
		return "", fmt.Errorf("could not create HAR file: %v", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(har); err != nil {
		return "", fmt.Errorf("error writing HAR file: %v", err)
	}

	return fullPath, nil
}

// ExportToCurl exports logs as a shell script with curl commands
// side can be "client" or "backend"
func (le *LogExporter) ExportToCurl(logs []models.RequestLog, side string, endpointName string) (string, error) {
	// Ensure export directory exists
	if err := os.MkdirAll(le.outputDir, 0755); err != nil {
		return "", fmt.Errorf("could not create export directory: %v", err)
	}

	// Generate filename with timestamp
	filename := fmt.Sprintf("request_logs_%s_%s.sh", side, time.Now().Format("20060102_150405"))
	fullPath := filepath.Join(le.outputDir, filename)

	file, err := os.Create(fullPath)
	if err != nil {
		return "", fmt.Errorf("could not create curl script file: %v", err)
	}
	defer file.Close()

	// Write script header
	fmt.Fprintf(file, "#!/bin/bash\n")
	fmt.Fprintf(file, "# Exported from Mockelot - %s\n", time.Now().Format(time.RFC3339))
	if endpointName != "" {
		fmt.Fprintf(file, "# Endpoint: %s\n", endpointName)
	}
	fmt.Fprintf(file, "# Side: %s\n", side)
	fmt.Fprintf(file, "#\n")
	fmt.Fprintf(file, "# Total requests: %d\n", len(logs))
	fmt.Fprintf(file, "\n")

	// Write curl commands for each log
	for i, log := range logs {
		var method, fullURL, path, body string
		var headers map[string][]string

		// Select client or backend side
		if side == "backend" && log.BackendRequest != nil {
			method = log.BackendRequest.Method
			fullURL = log.BackendRequest.FullURL
			path = log.BackendRequest.Path
			headers = log.BackendRequest.Headers
			body = log.BackendRequest.Body
		} else {
			method = log.ClientRequest.Method
			fullURL = log.ClientRequest.FullURL
			path = log.ClientRequest.Path
			headers = log.ClientRequest.Headers
			body = log.ClientRequest.Body
		}

		fmt.Fprintf(file, "# Request %d - %s %s\n", i+1, method, path)
		fmt.Fprintf(file, "curl -X %s '%s'", method, escapeSingleQuote(fullURL))

		// Add headers
		for key, values := range headers {
			// Skip certain headers that curl adds automatically
			if key == "Host" || key == "User-Agent" || key == "Accept-Encoding" {
				continue
			}
			for _, value := range values {
				fmt.Fprintf(file, " \\\n  -H '%s: %s'", escapeSingleQuote(key), escapeSingleQuote(value))
			}
		}

		// Add request body if present
		if body != "" {
			// Escape body for shell
			escapedBody := escapeSingleQuote(body)
			fmt.Fprintf(file, " \\\n  -d '%s'", escapedBody)
		}

		fmt.Fprintf(file, "\n\n")
	}

	// Make script executable
	if err := os.Chmod(fullPath, 0755); err != nil {
		return fullPath, fmt.Errorf("could not make script executable: %v", err)
	}

	return fullPath, nil
}

// escapeSingleQuote escapes single quotes for bash single-quoted strings
func escapeSingleQuote(s string) string {
	return strings.ReplaceAll(s, "'", "'\"'\"'")
}