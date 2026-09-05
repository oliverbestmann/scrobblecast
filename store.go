package main

import (
	"database/sql"
	"time"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

func OpenStore(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}

	const schema = `
CREATE TABLE IF NOT EXISTS plays (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	device TEXT NOT NULL,
	track TEXT NOT NULL,
	artist TEXT NOT NULL,
	album TEXT NOT NULL,
	played_at TIMESTAMP NOT NULL
);
`
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, err
	}

	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) RecordPlay(device string, np NowPlaying, at time.Time) error {
	_, err := s.db.Exec(
		`INSERT INTO plays (device, track, artist, album, played_at) VALUES (?, ?, ?, ?, ?)`,
		device, np.Track, np.Artist, np.Album, at,
	)
	return err
}
