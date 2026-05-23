package main

import (
	"log"
	"net/http"
	"os"
	"photo-video-album/internal/handler"
)

func main() {
	writeKey := os.Getenv("WRITE_KEY")
	if writeKey == "" {
		log.Println("WRITE_KEY not set — write mode disabled")
	}

	h := handler.New("media", "templates", "cache", writeKey)
	go h.PreGenThumbnails()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", h.Index)
	mux.HandleFunc("GET /year/{year}", h.YearView)
	mux.HandleFunc("GET /album/{year}/{name}", h.Album)
	mux.HandleFunc("GET /thumb/progress", h.ThumbProgress)
	mux.HandleFunc("GET /thumb/{year}/{album}/{file}", h.Thumb)
	mux.HandleFunc("POST /rotate/{year}/{album}/{file}", h.Rotate)
	mux.HandleFunc("POST /delete/{year}/{album}/{file}", h.Delete)
	mux.Handle("GET /media/", http.StripPrefix("/media/", http.FileServer(http.Dir("media"))))

	addr := ":8080"
	log.Printf("listening on http://localhost%s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}
