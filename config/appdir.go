package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

// GetAppDataDir returns the platform-specific application data directory.
// This follows platform conventions:
//   - Windows: %APPDATA%\Mockelot (e.g., C:\Users\Bob\AppData\Roaming\Mockelot)
//   - macOS: ~/Library/Application Support/Mockelot
//   - Linux: ~/.mockelot (or $XDG_CONFIG_HOME/mockelot if set)
//
// The directory is created if it doesn't exist.
func GetAppDataDir() (string, error) {
	var appDir string

	switch runtime.GOOS {
	case "windows":
		// Use APPDATA environment variable (standard for Windows apps)
		appData := os.Getenv("APPDATA")
		if appData == "" {
			// Fallback: try to construct from USERPROFILE
			userProfile := os.Getenv("USERPROFILE")
			if userProfile == "" {
				// Last resort: use os.UserHomeDir()
				home, err := os.UserHomeDir()
				if err != nil {
					return "", fmt.Errorf("cannot determine Windows app data directory: APPDATA, USERPROFILE not set, UserHomeDir failed: %w", err)
				}
				appData = filepath.Join(home, "AppData", "Roaming")
			} else {
				appData = filepath.Join(userProfile, "AppData", "Roaming")
			}
		}
		appDir = filepath.Join(appData, "Mockelot")

	case "darwin":
		// macOS: use ~/Library/Application Support
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("cannot determine macOS home directory: %w", err)
		}
		appDir = filepath.Join(home, "Library", "Application Support", "Mockelot")

	default:
		// Linux and other Unix-like systems
		// Prefer XDG_CONFIG_HOME if set, otherwise use ~/.mockelot
		if xdgConfig := os.Getenv("XDG_CONFIG_HOME"); xdgConfig != "" {
			appDir = filepath.Join(xdgConfig, "mockelot")
		} else {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", fmt.Errorf("cannot determine home directory: %w", err)
			}
			appDir = filepath.Join(home, ".mockelot")
		}
	}

	// Create directory if it doesn't exist
	if err := os.MkdirAll(appDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create app data directory %s: %w", appDir, err)
	}

	return appDir, nil
}

// GetCertsDir returns the certificate storage directory within the app data directory.
// The directory is created if it doesn't exist.
func GetCertsDir() (string, error) {
	appDir, err := GetAppDataDir()
	if err != nil {
		return "", err
	}

	certsDir := filepath.Join(appDir, "certs")
	if err := os.MkdirAll(certsDir, 0700); err != nil {
		return "", fmt.Errorf("failed to create certs directory %s: %w", certsDir, err)
	}

	return certsDir, nil
}

// GetLogFilePath returns the path to the application log file.
func GetLogFilePath() (string, error) {
	appDir, err := GetAppDataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(appDir, "mockelot.log"), nil
}

// GetRecentFilesPath returns the path to the recent files JSON file.
func GetRecentFilesPath() (string, error) {
	appDir, err := GetAppDataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(appDir, "recent-files.json"), nil
}

// GetLegacyConfigPath returns the path to the old server-config.yaml file
// (used for migration from older versions).
func GetLegacyConfigPath() (string, error) {
	appDir, err := GetAppDataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(appDir, "server-config.yaml"), nil
}

// WriteStartupError writes a startup error to a known location for debugging.
// This is especially useful on Windows where GUI apps don't show stderr.
func WriteStartupError(err error) {
	errorMsg := fmt.Sprintf("Mockelot startup failed at %s\nError: %v\nGOOS: %s, GOARCH: %s\n",
		time.Now().Format(time.RFC3339), err, runtime.GOOS, runtime.GOARCH)

	// Try to write to multiple locations
	locations := []string{}

	// Try app data dir first
	if appDir, e := GetAppDataDir(); e == nil {
		locations = append(locations, filepath.Join(appDir, "startup-error.txt"))
	}

	// Try temp directory
	locations = append(locations, filepath.Join(os.TempDir(), "mockelot-startup-error.txt"))

	// Try current directory as last resort
	locations = append(locations, "mockelot-startup-error.txt")

	for _, loc := range locations {
		if f, e := os.Create(loc); e == nil {
			f.WriteString(errorMsg)
			f.Close()
			break
		}
	}

	// Also write to stderr (may not be visible on Windows GUI apps)
	fmt.Fprintf(os.Stderr, "%s", errorMsg)
}
