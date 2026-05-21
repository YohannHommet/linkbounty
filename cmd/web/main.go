package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"linkbounty/internal/database"
	"linkbounty/internal/handlers"
)

func main() {
	dbPath := envOr("DB_PATH", "linkbounty.db")
	addr := envOr("ADDR", ":8080")
	uiDir := envOr("UI_DIR", "./ui/html")

	db, err := database.New(dbPath)
	if err != nil {
		log.Fatalf("database: %v", err)
	}

	// recover orphaned jobs from previous crash
	if err := db.MarkOrphansError(); err != nil {
		log.Printf("warn: mark orphans: %v", err)
	}

	registry := &handlers.JobRegistry{}

	app, err := handlers.NewApp(db, registry, uiDir)
	if err != nil {
		log.Fatalf("app init: %v", err)
	}

	// hourly cleanup of expired reports (7-day TTL)
	go func() {
		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			if err := db.Cleanup(7 * 24 * time.Hour); err != nil {
				log.Printf("warn: cleanup: %v", err)
			}
		}
	}()

	srv := &http.Server{
		Addr:         addr,
		Handler:      app.Routes(),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 120 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		fmt.Printf("LinkBounty listening on %s\n", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()

	<-quit
	log.Println("Shutting down...")

	// mark running jobs as error before closing
	if err := db.MarkOrphansError(); err != nil {
		log.Printf("warn: shutdown mark orphans: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("shutdown: %v", err)
	}
	log.Println("Done.")
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
