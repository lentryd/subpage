package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"subpage/internal/api"
	"subpage/internal/config"
)

var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

func main() {
	port := flag.String("port", "3000", "Port to listen on")
	noWeb := flag.Bool("no-web", false, "Disable serving the web UI (API-only mode)")
	debug := flag.Bool("debug", false, "Enable debug logging")
	version := flag.Bool("version", false, "Print version and exit")
	flag.Parse()

	if *version {
		fmt.Printf("subpage %s (commit: %s, built: %s)\n", Version, Commit, Date)
		return
	}

	level := slog.LevelInfo
	if *debug {
		level = slog.LevelDebug
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})))

	cfg := config.New(*port, *noWeb, *debug)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	slog.Info("starting", "version", Version, "commit", Commit)
	if err := api.New(cfg).Start(ctx); err != nil {
		log.Fatal(err)
	}
}
