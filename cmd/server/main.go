package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"time"

	"feed-ai/internal/logging"
	"feed-ai/internal/media"
	"feed-ai/internal/server"
)

func main() {
	addr := flag.String("addr", ":8080", "HTTP listen address")
	contentDir := flag.String("content-dir", "test-content", "media content directory")
	staticDir := flag.String("static-dir", "web/dist", "built frontend directory")
	flag.Parse()

	logger := log.New(logging.NewConsoleWriter(os.Stdout), "", 0)
	library := media.NewLibraryWithLogger(*contentDir, logger)
	app := server.New(library, *contentDir, *staticDir, logger)

	httpServer := &http.Server{
		Addr:              *addr,
		Handler:           app.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	logger.Printf("server starting addr=%s contentDir=%q staticDir=%q", *addr, *contentDir, *staticDir)
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Printf("server stopped error=%v", err)
		os.Exit(1)
	}
}
