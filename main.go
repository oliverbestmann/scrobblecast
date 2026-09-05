package main

import (
	"context"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	dbPath := flag.String("db", "scrobblecast.db", "path to the sqlite database")
	listenAddr := flag.String("listen", ":8080", "address to serve the top songs page on")
	flag.Parse()

	store, err := OpenStore(*dbPath)
	if err != nil {
		slog.Error("open store", slog.Any("error", err))
		os.Exit(1)
	}
	defer store.Close()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	server := &http.Server{Addr: *listenAddr, Handler: topSongsHandler(store)}
	go func() {
		<-ctx.Done()
		server.Close()
	}()

	go func() {
		slog.Info("starting http server", slog.String("addr", *listenAddr))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("http server", slog.Any("error", err))
		}
	}()

	NewPoller(store).Run(ctx)
}
