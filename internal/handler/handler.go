package handler

import (
	"html/template"
	"io/fs"
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

type Year struct {
	Name       string
	URLName    string
	AlbumCount int
	ThumbPath  string // first image found across albums
}

type Album struct {
	Name      string
	URLName   string
	Year      string
	URLYear   string
	Count     int
	ThumbPath string
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

// Index lists all year-level directories under media/.
func (h *Handler) Index(w http.ResponseWriter, r *http.Request) {
	entries, err := os.ReadDir(h.mediaDir)
	if err != nil {
		http.Error(w, "cannot read media directory", http.StatusInternalServerError)
		return
	}

	var years []Year
	for _, e := range entries {
		if !isDir(h.mediaDir, e) {
			continue
		}
		years = append(years, buildYear(h.mediaDir, e.Name()))
	}
	slices.Reverse(years) // newest year first

	h.render(w, "index", map[string]any{"Years": years})
}

// YearView lists album directories within a single year.
func (h *Handler) YearView(w http.ResponseWriter, r *http.Request) {
	year := filepath.Base(r.PathValue("year"))
	yearPath := filepath.Join(h.mediaDir, year)

	entries, err := os.ReadDir(yearPath)
	if err != nil {
		http.Error(w, "year not found", http.StatusNotFound)
		return
	}

	var albums []Album
	for _, e := range entries {
		if !isDir(yearPath, e) {
			continue
		}
		albums = append(albums, buildAlbum(h.mediaDir, year, e.Name()))
	}
	slices.Reverse(albums)

	h.render(w, "year", map[string]any{
		"Year":    year,
		"URLYear": url.PathEscape(year),
		"Albums":  albums,
	})
}

// Album lists media files within a single album.
func (h *Handler) Album(w http.ResponseWriter, r *http.Request) {
	year := filepath.Base(r.PathValue("year"))
	name := filepath.Base(r.PathValue("name"))
	albumPath := filepath.Join(h.mediaDir, year, name)

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
			Path: mediaPath(year, name, n),
			Type: kind,
		})
	}

	h.render(w, "album", map[string]any{
		"Year":      year,
		"URLYear":   url.PathEscape(year),
		"AlbumName": name,
		"Files":     files,
	})
}

func buildYear(mediaDir, name string) Year {
	y := Year{Name: name, URLName: url.PathEscape(name)}
	yearPath := filepath.Join(mediaDir, name)

	entries, err := os.ReadDir(yearPath)
	if err != nil {
		return y
	}
	for _, e := range entries {
		if !isDir(yearPath, e) {
			continue
		}
		y.AlbumCount++
		if y.ThumbPath == "" {
			if a := buildAlbum(mediaDir, name, e.Name()); a.ThumbPath != "" {
				y.ThumbPath = a.ThumbPath
			}
		}
	}
	return y
}

func buildAlbum(mediaDir, year, name string) Album {
	a := Album{
		Name:    name,
		URLName: url.PathEscape(name),
		Year:    year,
		URLYear: url.PathEscape(year),
	}
	entries, err := os.ReadDir(filepath.Join(mediaDir, year, name))
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
			a.ThumbPath = mediaPath(year, name, e.Name())
		}
	}
	return a
}

func mediaPath(year, album, file string) string {
	return "/media/" + url.PathEscape(year) + "/" + url.PathEscape(album) + "/" + url.PathEscape(file)
}

// isDir reports whether the entry is a directory, following symlinks.
func isDir(parent string, e os.DirEntry) bool {
	if e.Type()&fs.ModeSymlink == 0 {
		return e.IsDir()
	}
	info, err := os.Stat(filepath.Join(parent, e.Name()))
	return err == nil && info.IsDir()
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
