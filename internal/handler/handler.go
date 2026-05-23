package handler

import (
	"html/template"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

type Handler struct {
	mediaDir    string
	templateDir string
}

type Album struct {
	Name      string
	URLName   string // url-encoded, safe for use in href
	Count     int
	ThumbPath string // first image found, empty if none
}

type MediaFile struct {
	Name string
	Path string
	Type string // "image" or "video"
}

func New(mediaDir, templateDir string) *Handler {
	return &Handler{mediaDir: mediaDir, templateDir: templateDir}
}

func (h *Handler) render(w http.ResponseWriter, page string, data any) {
	tmpl, err := template.ParseFiles(
		filepath.Join(h.templateDir, "base.html"),
		filepath.Join(h.templateDir, page+".html"),
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := tmpl.ExecuteTemplate(w, "base", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (h *Handler) Index(w http.ResponseWriter, r *http.Request) {
	entries, err := os.ReadDir(h.mediaDir)
	if err != nil {
		http.Error(w, "cannot read media directory", http.StatusInternalServerError)
		return
	}

	var albums []Album
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		album := buildAlbum(h.mediaDir, e.Name())
		albums = append(albums, album)
	}

	// newest first — names are date-prefixed so reverse lexicographic works
	slices.Reverse(albums)

	h.render(w, "index", map[string]any{"Albums": albums})
}

func (h *Handler) Album(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")

	// guard against path traversal
	albumPath := filepath.Join(h.mediaDir, filepath.Clean("/"+name))
	if !strings.HasPrefix(albumPath, filepath.Clean(h.mediaDir)+string(filepath.Separator)) {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	entries, err := os.ReadDir(albumPath)
	if err != nil {
		http.Error(w, "album not found", http.StatusNotFound)
		return
	}

	var files []MediaFile
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		n := e.Name()
		ext := strings.ToLower(filepath.Ext(n))
		kind := mediaKind(ext)
		if kind == "" {
			continue
		}
		files = append(files, MediaFile{
			Name: n,
			Path: "/media/" + url.PathEscape(name) + "/" + url.PathEscape(n),
			Type: kind,
		})
	}

	h.render(w, "album", map[string]any{
		"AlbumName": name,
		"Files":     files,
	})
}

func buildAlbum(mediaDir, name string) Album {
	a := Album{
		Name:    name,
		URLName: url.PathEscape(name),
	}

	entries, err := os.ReadDir(filepath.Join(mediaDir, name))
	if err != nil {
		return a
	}

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(e.Name()))
		if mediaKind(ext) != "" {
			a.Count++
		}
		if a.ThumbPath == "" && mediaKind(ext) == "image" {
			a.ThumbPath = "/media/" + url.PathEscape(name) + "/" + url.PathEscape(e.Name())
		}
	}

	return a
}

func mediaKind(ext string) string {
	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".avif":
		return "image"
	case ".mp4", ".webm", ".mov", ".avi", ".mkv":
		return "video"
	}
	return ""
}
