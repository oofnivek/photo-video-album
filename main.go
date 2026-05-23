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
	mux.HandleFunc("GET /thumb/{year}/{album}/{file}", h.Thumb)
	mux.HandleFunc("POST /rotate/{year}/{album}/{file}", h.Rotate)
	mediaHandler := http.StripPrefix("/media/", http.FileServer(http.Dir("media")))
	mux.Handle("GET /media/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache")
		mediaHandler.ServeHTTP(w, r)
	}))

	addr := ":8080"
	log.Printf("listening on http://localhost%s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}
