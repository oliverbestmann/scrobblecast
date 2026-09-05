package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	dbPath := flag.String("db", "scrobblecast.db", "path to the sqlite database")
	flag.Parse()

	store, err := OpenStore(*dbPath)
	if err != nil {
		slog.Error("open store", slog.Any("error", err))
		os.Exit(1)
	}
	defer store.Close()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	NewPoller(store).Run(ctx)
}
