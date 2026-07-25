package main

import (
	"embed"
	"flag"
	"fmt"
	"log"
	"os"
	"runtime"
	"runtime/debug"
	"time"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"mockelot/config"
)

func init() {
	// On Linux with NVIDIA (and some other GPU drivers), WebKit2GTK 2.44+
	// enables the DMABuf renderer and GPU compositing by default, which
	// causes the WebView to render as a blank/black window. Force software
	// rendering unless the caller has already set these vars explicitly.
	if runtime.GOOS == "linux" {
		if _, ok := os.LookupEnv("WEBKIT_DISABLE_DMABUF_RENDERER"); !ok {
			os.Setenv("WEBKIT_DISABLE_DMABUF_RENDERER", "1")
		}
		if _, ok := os.LookupEnv("WEBKIT_DISABLE_COMPOSITING_MODE"); !ok {
			os.Setenv("WEBKIT_DISABLE_COMPOSITING_MODE", "1")
		}
	}
}

// Command-line flags
var logRequestMatching = flag.Bool("log-request-matching", false, "Enable verbose logging for request matching (logs to mockelot-matching.log)")

//go:embed all:frontend/dist
var assets embed.FS

var logFile *os.File

func initLogging() error {
	// Get platform-specific log file path
	logPath, err := config.GetLogFilePath()
	if err != nil {
		return fmt.Errorf("failed to get log file path: %w", err)
	}

	// Open log file (append mode)
	logFile, err = os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file %s: %w", logPath, err)
	}

	// Set log output to file
	log.SetOutput(logFile)
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	// Log startup with diagnostic info
	log.Printf("=== Mockelot starting at %s ===", time.Now().Format(time.RFC3339))
	log.Printf("Platform: GOOS=%s, GOARCH=%s", runtime.GOOS, runtime.GOARCH)
	log.Printf("Log file: %s", logPath)

	// Log app data directory for debugging
	appDir, err := config.GetAppDataDir()
	if err != nil {
		log.Printf("WARNING: Could not get app data directory: %v", err)
	} else {
		log.Printf("App data directory: %s", appDir)
	}

	// Log relevant environment variables for debugging (Windows-specific)
	if runtime.GOOS == "windows" {
		log.Printf("Environment: APPDATA=%s", os.Getenv("APPDATA"))
		log.Printf("Environment: USERPROFILE=%s", os.Getenv("USERPROFILE"))
	}

	return nil
}

func main() {
	// Parse command-line flags
	flag.Parse()

	// Initialize logging first
	if err := initLogging(); err != nil {
		// If logging fails, write error to known location and stderr
		config.WriteStartupError(fmt.Errorf("logging initialization failed: %w", err))
		os.Exit(1)
	}

	// Log if request matching debug is enabled
	if *logRequestMatching {
		log.Println("Request matching debug logging enabled (logs to mockelot-matching.log)")
	}
	defer func() {
		if logFile != nil {
			logFile.Close()
		}
	}()

	// Recover from panics
	defer func() {
		if r := recover(); r != nil {
			log.Printf("PANIC RECOVERED: %v\nStack trace:\n%s", r, debug.Stack())
			config.WriteStartupError(fmt.Errorf("panic: %v", r))
			if logFile != nil {
				logFile.Sync()
			}
			os.Exit(1)
		}
	}()

	// Create an instance of the app structure
	app := NewApp(*logRequestMatching)
	log.Println("App instance created")

	// Create application with options
	log.Println("Starting Wails application...")
	err := wails.Run(&options.App{
		Title:  "Mockelot",
		Width:  1400,
		Height: 900,
		MinWidth: 1200,
		MinHeight: 700,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 17, G: 24, B: 39, A: 1}, // Tailwind gray-900
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		log.Printf("ERROR: Wails application failed: %v", err)
		config.WriteStartupError(fmt.Errorf("Wails application failed: %w", err))
		if logFile != nil {
			logFile.Sync()
		}
		os.Exit(1)
	}

	log.Println("=== Mockelot shutdown complete ===")
}
