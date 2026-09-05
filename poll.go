package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/vishen/go-chromecast/application"
	pb "github.com/vishen/go-chromecast/cast/proto"
	"github.com/vishen/go-chromecast/dns"
)

const (
	musicTrackMetadataType = 3
	deviceCheckTimeout     = 5 * time.Second
	captureGracePeriod     = 1 * time.Second
)

// NowPlaying describes a currently playing music track on a chromecast device.
type NowPlaying struct {
	Track  string
	Artist string
	Album  string
}

func (n NowPlaying) signature() string {
	return n.Track + "\x1f" + n.Artist + "\x1f" + n.Album
}

// mediaStatusMessage mirrors the MEDIA_STATUS payload, including the
// albumName field that the go-chromecast library's own types don't expose.
type mediaStatusMessage struct {
	Type   string `json:"type"`
	Status []struct {
		PlayerState string `json:"playerState"`
		Media       struct {
			ContentId string `json:"contentId"`
			Metadata  struct {
				MetadataType int    `json:"metadataType"`
				Title        string `json:"title"`
				Artist       string `json:"artist"`
				AlbumName    string `json:"albumName"`
			} `json:"metadata"`
		} `json:"media"`
	} `json:"status"`
}

// Poller periodically checks all active chromecast devices for their
// currently playing entry and records changes to a Store.
type Poller struct {
	store *Store

	mu           sync.Mutex
	lastByDevice map[string]string
}

func NewPoller(store *Store) *Poller {
	return &Poller{
		store:        store,
		lastByDevice: map[string]string{},
	}
}

// Run polls every active chromecast device once a minute until ctx is done.
func (p *Poller) Run(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	p.tick(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.tick(ctx)
		}
	}
}

func (p *Poller) tick(ctx context.Context) {
	entries, err := discoverDevices(ctx)
	if err != nil {
		slog.Error("discover devices", slog.Any("error", err))
		return
	}

	var wg sync.WaitGroup
	for _, entry := range entries {
		entry := entry
		wg.Add(1)
		go func() {
			defer wg.Done()
			p.checkDevice(ctx, entry)
		}()
	}
	wg.Wait()
}

func discoverDevices(ctx context.Context) ([]dns.CastEntry, error) {
	discoverCtx, cancel := context.WithTimeout(ctx, deviceCheckTimeout)
	defer cancel()

	entryChan, err := dns.DiscoverCastDNSEntries(discoverCtx, nil)
	if err != nil {
		return nil, err
	}

	seen := map[string]dns.CastEntry{}
	for entry := range entryChan {
		seen[entry.GetUUID()] = entry
	}

	entries := make([]dns.CastEntry, 0, len(seen))
	for _, entry := range seen {
		entries = append(entries, entry)
	}
	return entries, nil
}

func (p *Poller) checkDevice(ctx context.Context, entry dns.CastEntry) {
	slog.Info("Checking device", slog.String("device", entry.GetName()))
	done := make(chan *NowPlaying, 1)
	go func() {
		done <- fetchNowPlaying(entry)
	}()

	var now *NowPlaying
	select {
	case now = <-done:
	case <-ctx.Done():
		return
	case <-time.After(deviceCheckTimeout):
		slog.Warn("device check timed out", slog.String("device", entry.GetName()), slog.Duration("timeout", deviceCheckTimeout))
		return
	}

	if now == nil {
		return
	}

	sig := now.signature()

	p.mu.Lock()
	changed := p.lastByDevice[entry.GetUUID()] != sig
	p.lastByDevice[entry.GetUUID()] = sig
	p.mu.Unlock()

	if !changed {
		return
	}

	if err := p.store.RecordPlay(entry.GetName(), *now, time.Now()); err != nil {
		slog.Error("record play", slog.String("device", entry.GetName()), slog.Any("error", err))
		return
	}

	slog.Info("recorded play",
		slog.String("device", entry.GetName()),
		slog.String("track", now.Track),
		slog.String("artist", now.Artist),
		slog.String("album", now.Album),
	)
}

// fetchNowPlaying connects to a single chromecast device and returns its
// currently playing music track, or nil if it isn't currently playing music.
func fetchNowPlaying(entry dns.CastEntry) *NowPlaying {
	app := application.NewApplication(application.WithConnectionRetries(1))

	captured := make(chan NowPlaying, 1)
	app.AddMessageFunc(func(msg *pb.CastMessage) {
		if msg.PayloadUtf8 == nil {
			return
		}

		var status mediaStatusMessage
		if err := json.Unmarshal([]byte(*msg.PayloadUtf8), &status); err != nil {
			return
		}
		if status.Type != "MEDIA_STATUS" {
			return
		}

		for _, s := range status.Status {
			meta := s.Media.Metadata
			if s.PlayerState != "PLAYING" || meta.MetadataType != musicTrackMetadataType || meta.Title == "" {
				continue
			}

			np := NowPlaying{
				Track:  meta.Title,
				Artist: meta.Artist,
				Album:  meta.AlbumName,
			}
			select {
			case captured <- np:
			default:
			}
		}
	})

	if err := app.Start(entry.GetAddr(), entry.GetPort()); err != nil {
		app.Close(false)
		return nil
	}
	defer app.Close(false)

	select {
	case np := <-captured:
		return &np
	case <-time.After(captureGracePeriod):
		return nil
	}
}
