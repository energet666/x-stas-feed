package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"time"

	"feed-ai/internal/media"
	"feed-ai/internal/server"
)

func main() {
	addr := flag.String("addr", ":8080", "HTTP listen address")
	contentDir := flag.String("content-dir", "test-content", "media content directory")
	staticDir := flag.String("static-dir", "web/dist", "built frontend directory")
	flag.Parse()

	logger := log.New(os.Stdout, "", log.LstdFlags)
	library := media.NewLibraryWithLogger(*contentDir, logger)
	app := server.New(library, *staticDir, logger)

	httpServer := &http.Server{
		Addr:              *addr,
		Handler:           app.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	logger.Printf("server starting addr=%s contentDir=%s staticDir=%s", *addr, *contentDir, *staticDir)
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Printf("server stopped error=%v", err)
		os.Exit(1)
	}
}
