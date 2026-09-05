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

// TopSong is an aggregated play count for a track/artist pair.
type TopSong struct {
	Track  string
	Artist string
	Album  string
	Plays  int
}

// TopSongs returns the most played tracks since the given time, ordered by
// play count descending, limited to limit entries.
func (s *Store) TopSongs(since time.Time, limit int) ([]TopSong, error) {
	rows, err := s.db.Query(
		`SELECT track, artist, album, COUNT(*) AS plays
		 FROM plays
		 WHERE played_at >= ?
		 GROUP BY track, artist
		 ORDER BY plays DESC, track ASC
		 LIMIT ?`,
		since, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var songs []TopSong
	for rows.Next() {
		var song TopSong
		if err := rows.Scan(&song.Track, &song.Artist, &song.Album, &song.Plays); err != nil {
			return nil, err
		}
		songs = append(songs, song)
	}
	return songs, rows.Err()
}
