package handler

import (
	"html/template"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
)

type Handler struct {
	mediaDir    string
	templateDir string
	cacheDir    string
}

type Year struct {
	Name       string
	URLName    string
	AlbumCount int
	ThumbPath  string
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
	Name      string
	Path      string // actual media URL (used by lightbox)
	ThumbPath string // thumbnail URL (image: same as Path, video: /thumb/...)
	Type      string // "image" or "video"
}

func New(mediaDir, templateDir, cacheDir string) *Handler {
	return &Handler{mediaDir: mediaDir, templateDir: templateDir, cacheDir: cacheDir}
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
	slices.Reverse(years)

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
		mp := mediaPath(year, name, n)
		tp := mp
		if kind == "video" {
			tp = thumbURL(year, name, n)
		}
		files = append(files, MediaFile{
			Name:      n,
			Path:      mp,
			ThumbPath: tp,
			Type:      kind,
		})
	}

	h.render(w, "album", map[string]any{
		"Year":      year,
		"URLYear":   url.PathEscape(year),
		"AlbumName": name,
		"Files":     files,
	})
}

// Thumb serves a video thumbnail, generating it on first request.
func (h *Handler) Thumb(w http.ResponseWriter, r *http.Request) {
	year := filepath.Base(r.PathValue("year"))
	album := filepath.Base(r.PathValue("album"))
	file := filepath.Base(r.PathValue("file"))

	videoPath := filepath.Join(h.mediaDir, year, album, file)
	thumbPath := filepath.Join(h.cacheDir, "thumbnails", year, album, file+".jpg")

	if _, err := os.Stat(thumbPath); os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Dir(thumbPath), 0755); err != nil {
			http.Error(w, "cache directory error", http.StatusInternalServerError)
			return
		}
		if err := generateThumb(videoPath, thumbPath); err != nil {
			log.Printf("thumb generation failed for %s: %v", videoPath, err)
			http.NotFound(w, r)
			return
		}
	}

	http.ServeFile(w, r, thumbPath)
}

func generateThumb(videoPath, thumbPath string) error {
	cmd := exec.Command("ffmpeg",
		"-i", videoPath,
		"-ss", "00:00:01",
		"-vframes", "1",
		"-vf", "scale=480:-1",
		"-q:v", "4",
		"-y",
		thumbPath,
	)
	return cmd.Run()
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

	var firstVideoThumb string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(e.Name()))
		kind := mediaKind(ext)
		if kind == "" {
			continue
		}
		a.Count++
		if a.ThumbPath == "" && kind == "image" {
			a.ThumbPath = mediaPath(year, name, e.Name())
		}
		if firstVideoThumb == "" && kind == "video" {
			firstVideoThumb = thumbURL(year, name, e.Name())
		}
	}
	if a.ThumbPath == "" {
		a.ThumbPath = firstVideoThumb
	}
	return a
}

func mediaPath(year, album, file string) string {
	return "/media/" + url.PathEscape(year) + "/" + url.PathEscape(album) + "/" + url.PathEscape(file)
}

func thumbURL(year, album, file string) string {
	return "/thumb/" + url.PathEscape(year) + "/" + url.PathEscape(album) + "/" + url.PathEscape(file)
}

// isDir reports whether the entry is a directory, following symlinks.
// os.Stat is used as a fallback to handle symlinks and filesystems that
// don't report entry types (FAT32, exFAT, some network mounts).
func isDir(parent string, e os.DirEntry) bool {
	if e.IsDir() {
		return true
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
