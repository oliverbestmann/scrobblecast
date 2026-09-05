package main

import (
	"html/template"
	"net/http"
	"time"
)

const topSongsWindow = 7 * 24 * time.Hour
const topSongsLimit = 30

var topSongsTemplate = template.Must(
	template.New("top-songs").
		Funcs(template.FuncMap{"inc": func(i int) int { return i + 1 }}).
		Parse(`<!DOCTYPE html>
<html>
<head>
	<meta charset="utf-8">
	<title>Top songs - last 7 days</title>
	<style>
		body { font-family: sans-serif; max-width: 60rem; margin: 2rem auto; }
		table { width: 100%; border-collapse: collapse; }
		th, td { text-align: left; padding: 0.3rem 0.6rem; border-bottom: 1px solid #ddd; }
		th { border-bottom: 2px solid #333; }
		td:last-child, th:last-child { text-align: right; }
	</style>
</head>
<body>
	<h1>Top songs - last 7 days</h1>
	<table>
		<tr><th>#</th><th>Track</th><th>Artist</th><th>Album</th><th>Plays</th></tr>
		{{range $i, $song := .Songs}}
		<tr>
			<td>{{inc $i}}</td>
			<td>{{$song.Track}}</td>
			<td>{{$song.Artist}}</td>
			<td>{{$song.Album}}</td>
			<td>{{$song.Plays}}</td>
		</tr>
		{{else}}
		<tr><td colspan="5">No plays recorded yet.</td></tr>
		{{end}}
	</table>
</body>
</html>
`))

func topSongsHandler(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		songs, err := store.TopSongs(time.Now().Add(-topSongsWindow), topSongsLimit)
		if err != nil {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = topSongsTemplate.Execute(w, struct{ Songs []TopSong }{Songs: songs})
	}
}
