package main

import (
	"log"
	"net/http"
	"photo-video-album/internal/handler"
)

func main() {
	h := handler.New("media", "templates", "cache")

	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", h.Index)
	mux.HandleFunc("GET /year/{year}", h.YearView)
	mux.HandleFunc("GET /album/{year}/{name}", h.Album)
	mux.HandleFunc("GET /thumb/{year}/{album}/{file}", h.Thumb)
	mux.Handle("GET /media/", http.StripPrefix("/media/", http.FileServer(http.Dir("media"))))

	addr := ":8080"
	log.Printf("listening on http://localhost%s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}
