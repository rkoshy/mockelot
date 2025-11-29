package config

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"mockelot/models"
)

const DefaultConfigFile = "config.json"

type ConfigManager struct {
	configPath string
	mutex      sync.RWMutex
}

func NewConfigManager(customPath string) *ConfigManager {
	if customPath == "" {
		customPath = DefaultConfigFile
	}
	return &ConfigManager{
		configPath: customPath,
	}
}

func (cm *ConfigManager) Load() (*models.AppConfig, error) {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	// If config file doesn't exist, return default configuration
	if _, err := os.Stat(cm.configPath); os.IsNotExist(err) {
		return &models.AppConfig{
			Port:         8080,
			Responses:    []models.MethodResponse{},
			LastModified: time.Now(),
		}, nil
	}

	file, err := os.Open(cm.configPath)
	if err != nil {
		return nil, fmt.Errorf("could not open config file: %v", err)
	}
	defer file.Close()

	var config models.AppConfig
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&config); err != nil {
		return nil, fmt.Errorf("could not decode config: %v", err)
	}

	return &config, nil
}

func (cm *ConfigManager) Save(config *models.AppConfig) error {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	// Ensure directory exists
	dir := filepath.Dir(cm.configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("could not create config directory: %v", err)
	}

	// Update last modified time
	config.LastModified = time.Now()

	// Create temporary file to ensure atomic write
	tempFile, err := os.CreateTemp(dir, "config-*.json")
	if err != nil {
		return fmt.Errorf("could not create temporary config file: %v", err)
	}
	defer os.Remove(tempFile.Name())

	// Encode and write to temp file
	encoder := json.NewEncoder(tempFile)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(config); err != nil {
		tempFile.Close()
		return fmt.Errorf("could not encode config: %v", err)
	}
	tempFile.Close()

	// Atomically replace config file
	if err := os.Rename(tempFile.Name(), cm.configPath); err != nil {
		return fmt.Errorf("could not replace config file: %v", err)
	}

	log.Println("Configuration saved successfully")
	return nil
}

// WatchConfigChanges provides a channel that receives configuration updates
func (cm *ConfigManager) WatchConfigChanges(interval time.Duration, onConfigChange func(*models.AppConfig)) {
	go func() {
		lastModified := time.Time{}
		for {
			time.Sleep(interval)

			file, err := os.Stat(cm.configPath)
			if err != nil {
				continue
			}

			if file.ModTime().After(lastModified) {
				config, err := cm.Load()
				if err != nil {
					log.Printf("Error loading updated config: %v", err)
					continue
				}

				lastModified = file.ModTime()
				onConfigChange(config)
			}
		}
	}()
}