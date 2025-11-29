package export

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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
			log.Timestamp.Format(time.RFC3339),
			log.Method,
			log.Path,
			log.SourceIP,
			log.UserAgent,
			log.Protocol,
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