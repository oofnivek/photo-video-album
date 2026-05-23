package handler

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
)

type Handler struct {
	mediaDir    string
	templateDir string
	cacheDir    string
	writeKey    string

	thumbMu      sync.Mutex
	thumbTotal   int
	thumbDone    int
	thumbRunning bool
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
	ThumbPath string // thumbnail URL (/thumb/...)
	RotateURL string // rotation endpoint; empty for videos
	Type      string // "image" or "video"
}

func New(mediaDir, templateDir, cacheDir, writeKey string) *Handler {
	return &Handler{
		mediaDir:    mediaDir,
		templateDir: templateDir,
		cacheDir:    cacheDir,
		writeKey:    writeKey,
	}
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

// isWriteMode returns true if the request carries a valid write key via
// query param or cookie.
func (h *Handler) isWriteMode(r *http.Request) bool {
	if h.writeKey == "" {
		return false
	}
	if r.URL.Query().Get("key") == h.writeKey {
		return true
	}
	if c, err := r.Cookie("write_access"); err == nil {
		return c.Value == h.writeKey
	}
	return false
}

// maybeSetWriteCookie sets a session cookie when a valid key is supplied via
// query param, so subsequent requests don't need to repeat the param.
func (h *Handler) maybeSetWriteCookie(w http.ResponseWriter, r *http.Request) {
	if h.writeKey == "" {
		return
	}
	if r.URL.Query().Get("key") == h.writeKey {
		http.SetCookie(w, &http.Cookie{
			Name:     "write_access",
			Value:    h.writeKey,
			Path:     "/",
			HttpOnly: true,
			SameSite: http.SameSiteStrictMode,
		})
	}
}

// Index lists all year-level directories under media/.
func (h *Handler) Index(w http.ResponseWriter, r *http.Request) {
	h.maybeSetWriteCookie(w, r)

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

	h.render(w, "index", map[string]any{
		"Years":     years,
		"WriteMode": h.isWriteMode(r),
	})
}

// YearView lists album directories within a single year.
func (h *Handler) YearView(w http.ResponseWriter, r *http.Request) {
	h.maybeSetWriteCookie(w, r)

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
		"Year":      year,
		"URLYear":   url.PathEscape(year),
		"Albums":    albums,
		"WriteMode": h.isWriteMode(r),
	})
}

// Album lists media files within a single album.
func (h *Handler) Album(w http.ResponseWriter, r *http.Request) {
	h.maybeSetWriteCookie(w, r)

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
		rURL := ""
		tPath := thumbURL(year, name, n)
		if kind == "image" {
			rURL = rotateURL(year, name, n)
			if info, err := e.Info(); err == nil {
				tPath = fmt.Sprintf("%s?t=%d", tPath, info.ModTime().Unix())
			}
		}
		files = append(files, MediaFile{
			Name:      n,
			Path:      mediaPath(year, name, n),
			ThumbPath: tPath,
			RotateURL: rURL,
			Type:      kind,
		})
	}

	h.render(w, "album", map[string]any{
		"Year":      year,
		"URLYear":   url.PathEscape(year),
		"AlbumName": name,
		"Files":     files,
		"WriteMode": h.isWriteMode(r),
	})
}

// Thumb serves a thumbnail for images and videos, generating it on first request.
func (h *Handler) Thumb(w http.ResponseWriter, r *http.Request) {
	year := filepath.Base(r.PathValue("year"))
	album := filepath.Base(r.PathValue("album"))
	file := filepath.Base(r.PathValue("file"))

	kind := mediaKind(strings.ToLower(filepath.Ext(file)))
	if kind == "" {
		http.NotFound(w, r)
		return
	}

	sourcePath := filepath.Join(h.mediaDir, year, album, file)
	thumbPath := filepath.Join(h.cacheDir, "thumbnails", year, album, file+".jpg")

	if _, err := os.Stat(thumbPath); os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Dir(thumbPath), 0755); err != nil {
			http.Error(w, "cache directory error", http.StatusInternalServerError)
			return
		}
		log.Printf("generating thumbnail: %s", sourcePath)
		if err := generateThumb(sourcePath, thumbPath, kind); err != nil {
			log.Printf("thumb generation failed for %s: %v", sourcePath, err)
			http.NotFound(w, r)
			return
		}
	}

	http.ServeFile(w, r, thumbPath)
}

// Rotate rotates an image 90° CW or CCW, updates the file in place, and
// returns an updated <img> fragment for HTMX to swap into the page.
func (h *Handler) Rotate(w http.ResponseWriter, r *http.Request) {
	if !h.isWriteMode(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	year := filepath.Base(r.PathValue("year"))
	album := filepath.Base(r.PathValue("album"))
	file := filepath.Base(r.PathValue("file"))
	dir := r.URL.Query().Get("dir") // "cw" or "ccw"

	filePath := filepath.Join(h.mediaDir, year, album, file)
	log.Printf("rotate: %s dir=%s", filePath, dir)

	if err := rotateImage(filePath, dir); err != nil {
		log.Printf("rotate failed for %s: %v", filePath, err)
		http.Error(w, "rotation failed", http.StatusInternalServerError)
		return
	}
	log.Printf("rotate: done %s", filePath)

	// Invalidate cached thumbnail so the next /thumb request regenerates it.
	thumbPath := filepath.Join(h.cacheDir, "thumbnails", year, album, file+".jpg")
	if err := os.Remove(thumbPath); err != nil && !os.IsNotExist(err) {
		log.Printf("rotate: failed to remove cached thumbnail %s: %v", thumbPath, err)
	}

	// Return a cache-busted <img> for HTMX to swap in.
	newSrc := fmt.Sprintf("%s?t=%d", thumbURL(year, album, file), time.Now().UnixMilli())
	fmt.Fprintf(w,
		`<img src="%s" alt="%s" loading="lazy" style="object-fit: cover; width: 100%%; height: 100%%;">`,
		newSrc, file,
	)
}

type thumbJob struct {
	src, thumb, kind string
}

// PreGenThumbnails walks all media in descending order and generates any missing
// thumbnails. Intended to run in a background goroutine at startup.
func (h *Handler) PreGenThumbnails() {
	jobs := h.collectMissingThumbs()

	h.thumbMu.Lock()
	h.thumbTotal = len(jobs)
	h.thumbDone = 0
	h.thumbRunning = len(jobs) > 0
	h.thumbMu.Unlock()

	if len(jobs) == 0 {
		log.Println("pregen: all thumbnails up to date")
		return
	}

	log.Printf("pregen: %d thumbnails to generate", len(jobs))
	for _, j := range jobs {
		log.Printf("generating thumbnail: %s", j.src)
		if err := generateThumb(j.src, j.thumb, j.kind); err != nil {
			log.Printf("pregen: thumb generation failed for %s: %v", j.src, err)
		}
		h.thumbMu.Lock()
		h.thumbDone++
		h.thumbMu.Unlock()
	}

	h.thumbMu.Lock()
	h.thumbRunning = false
	h.thumbMu.Unlock()
	log.Println("pregen: all thumbnails up to date")
}

func (h *Handler) collectMissingThumbs() []thumbJob {
	var jobs []thumbJob
	years, err := os.ReadDir(h.mediaDir)
	if err != nil {
		log.Printf("pregen: cannot read media dir: %v", err)
		return nil
	}
	slices.Reverse(years)
	for _, y := range years {
		if !isDir(h.mediaDir, y) {
			continue
		}
		yearPath := filepath.Join(h.mediaDir, y.Name())
		albums, err := os.ReadDir(yearPath)
		if err != nil {
			continue
		}
		slices.Reverse(albums)
		for _, a := range albums {
			if !isDir(yearPath, a) {
				continue
			}
			albumPath := filepath.Join(yearPath, a.Name())
			files, err := os.ReadDir(albumPath)
			if err != nil {
				continue
			}
			slices.Reverse(files)
			for _, f := range files {
				if f.IsDir() {
					continue
				}
				kind := mediaKind(strings.ToLower(filepath.Ext(f.Name())))
				if kind == "" {
					continue
				}
				src := filepath.Join(albumPath, f.Name())
				thumb := filepath.Join(h.cacheDir, "thumbnails", y.Name(), a.Name(), f.Name()+".jpg")
				if _, err := os.Stat(thumb); err == nil {
					continue
				}
				if err := os.MkdirAll(filepath.Dir(thumb), 0755); err != nil {
					log.Printf("pregen: mkdir failed: %v", err)
					continue
				}
				jobs = append(jobs, thumbJob{src: src, thumb: thumb, kind: kind})
			}
		}
	}
	return jobs
}

// ThumbProgress returns an HTML fragment showing thumbnail generation progress.
// When generation is idle it returns an empty div (no hx-trigger) so polling stops.
func (h *Handler) ThumbProgress(w http.ResponseWriter, r *http.Request) {
	h.thumbMu.Lock()
	total := h.thumbTotal
	done := h.thumbDone
	running := h.thumbRunning
	h.thumbMu.Unlock()

	w.Header().Set("Content-Type", "text/html")
	if !running {
		fmt.Fprint(w, `<div id="thumb-progress"></div>`)
		return
	}

	pct := 0
	if total > 0 {
		pct = done * 100 / total
	}
	fmt.Fprintf(w,
		`<div id="thumb-progress" hx-get="/thumb/progress" hx-trigger="every 3s" hx-swap="outerHTML"`+
			` style="padding:0.4rem 1.5rem; background:#fffbe6; border-bottom:1px solid #ffe58f;">`+
			`<p style="font-size:0.82rem; margin-bottom:0.2rem;">Generating thumbnails: %d / %d</p>`+
			`<progress class="progress is-warning is-small" value="%d" max="%d" style="margin-bottom:0;">%d%%</progress>`+
			`</div>`,
		done, total, done, total, pct)
}

func generateThumb(sourcePath, thumbPath, kind string) error {
	args := []string{"-i", sourcePath}
	if kind == "video" {
		args = append(args, "-ss", "00:00:01", "-vframes", "1")
	}
	args = append(args, "-vf", "scale=480:-1", "-q:v", "4", "-y", thumbPath)
	cmd := exec.Command("ffmpeg", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%w\nffmpeg output:\n%s", err, out)
	}
	return nil
}

func rotateImage(filePath, dir string) error {
	var transpose string
	switch dir {
	case "cw":
		transpose = "transpose=1"
	case "ccw":
		transpose = "transpose=2"
	default:
		return fmt.Errorf("invalid direction: %s", dir)
	}

	tmp := filePath + ".tmp.jpg"
	cmd := exec.Command("ffmpeg", "-i", filePath, "-vf", transpose, "-y", tmp)
	if out, err := cmd.CombinedOutput(); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("%w\nffmpeg output:\n%s", err, out)
	}
	if err := os.Rename(tmp, filePath); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("rename tmp to original: %w", err)
	}
	return nil
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
			p := thumbURL(year, name, e.Name())
			if info, err := e.Info(); err == nil {
				p = fmt.Sprintf("%s?t=%d", p, info.ModTime().Unix())
			}
			a.ThumbPath = p
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

func rotateURL(year, album, file string) string {
	return "/rotate/" + url.PathEscape(year) + "/" + url.PathEscape(album) + "/" + url.PathEscape(file)
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
