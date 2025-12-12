package config

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
	"mockelot/models"
)

const DefaultServerConfigFile = "server-config.yaml"

type ServerConfigManager struct {
	configPath string
	mutex      sync.RWMutex
}

func NewServerConfigManager(customPath string) *ServerConfigManager {
	if customPath == "" {
		// Use user's home directory
		homeDir, err := os.UserHomeDir()
		if err != nil {
			log.Printf("Could not determine home directory, using current directory: %v", err)
			customPath = DefaultServerConfigFile
		} else {
			// Store in ~/.mockelot/server-config.yaml
			mockelotDir := filepath.Join(homeDir, ".mockelot")
			customPath = filepath.Join(mockelotDir, DefaultServerConfigFile)
		}
	}
	return &ServerConfigManager{
		configPath: customPath,
	}
}

// Load loads server configuration from disk
func (scm *ServerConfigManager) Load() (*models.ServerConfig, error) {
	scm.mutex.RLock()
	defer scm.mutex.RUnlock()

	// If config file doesn't exist, return default configuration
	if _, err := os.Stat(scm.configPath); os.IsNotExist(err) {
		return &models.ServerConfig{
			Port:      8080,
			HTTPSPort: 8443,
			CertMode:  models.CertModeAuto,
		}, nil
	}

	file, err := os.Open(scm.configPath)
	if err != nil {
		return nil, fmt.Errorf("could not open server config file: %v", err)
	}
	defer file.Close()

	var config models.ServerConfig
	decoder := yaml.NewDecoder(file)
	if err := decoder.Decode(&config); err != nil {
		return nil, fmt.Errorf("could not decode server config: %v", err)
	}

	// Apply default migrations
	if config.HTTPSPort == 0 {
		config.HTTPSPort = 8443
	}
	if config.CertMode == "" {
		config.CertMode = models.CertModeAuto
	}

	return &config, nil
}

// Save saves server configuration to disk
func (scm *ServerConfigManager) Save(config *models.ServerConfig) error {
	scm.mutex.Lock()
	defer scm.mutex.Unlock()

	// Ensure directory exists
	dir := filepath.Dir(scm.configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("could not create config directory: %v", err)
	}

	// Update last modified time
	config.LastModified = time.Now()

	// Create temporary file to ensure atomic write
	tempFile, err := os.CreateTemp(dir, "server-config-*.yaml")
	if err != nil {
		return fmt.Errorf("could not create temporary config file: %v", err)
	}
	defer os.Remove(tempFile.Name())

	// Encode and write to temp file
	encoder := yaml.NewEncoder(tempFile)
	encoder.SetIndent(2)
	if err := encoder.Encode(config); err != nil {
		tempFile.Close()
		return fmt.Errorf("could not encode server config: %v", err)
	}
	tempFile.Close()

	// Atomically replace config file
	if err := os.Rename(tempFile.Name(), scm.configPath); err != nil {
		return fmt.Errorf("could not replace server config file: %v", err)
	}

	log.Println("Server configuration saved successfully to", scm.configPath)
	return nil
}
