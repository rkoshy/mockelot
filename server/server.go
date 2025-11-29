package server

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"mockelot/models"
)

type HTTPServer struct {
	server        *http.Server
	config        *models.AppConfig
	configMutex   sync.RWMutex
	requestLogger RequestLogger
	stopChan      chan struct{}
}

func NewHTTPServer(config *models.AppConfig, requestLogger RequestLogger) *HTTPServer {
	return &HTTPServer{
		config:        config,
		requestLogger: requestLogger,
		stopChan:      make(chan struct{}),
	}
}

func (s *HTTPServer) Start() error {
	// Thread-safe config access
	s.configMutex.RLock()
	port := s.config.Port
	s.configMutex.RUnlock()

	handler := NewResponseHandler(s.config, s.requestLogger)

	// Create HTTP server
	s.server = &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		Handler:      http.HandlerFunc(handler.HandleRequest),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	// Start server in a goroutine
	go func() {
		log.Printf("Starting server on port %d", port)
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("Server error: %v", err)
		}
		s.stopChan <- struct{}{}
	}()

	return nil
}

func (s *HTTPServer) Stop() error {
	if s.server == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.server.Shutdown(ctx); err != nil {
		log.Printf("Server shutdown error: %v", err)
		return err
	}

	<-s.stopChan
	log.Println("Server stopped")
	return nil
}

func (s *HTTPServer) UpdateConfig(newConfig *models.AppConfig) {
	s.configMutex.Lock()
	defer s.configMutex.Unlock()
	s.config = newConfig
}