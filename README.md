# Photo & Video Album

A simple local photo and video album viewer. Drop media files into the `media/` folder and browse them in your browser.

## Requirements

- [Go](https://go.dev/dl/) 1.22 or later
- [ffmpeg](https://ffmpeg.org/download.html) — for video thumbnail generation

## Getting started

```bash
# Clone the repository
git clone https://github.com/your-username/photo-video-album.git
cd photo-video-album

# Run the server
make run
```

Then open [http://localhost:8080](http://localhost:8080) in your browser.

## Adding media

Organise files in a two-level hierarchy: year folders at the top, named album folders inside each year:

```
media/
├── 2025/
│   └── 2025-07-10 Summer Road Trip/
│       ├── photo1.jpg
│       └── clip.mp4
└── 2026/
    └── 2026-12-25 Skiing in Switzerland/
        ├── family.jpg
        └── highlight.mp4
```

The home page lists years → clicking a year shows its albums → clicking an album shows the media. Refresh after adding files — no server restart needed.
    
**Supported formats**

| Type   | Extensions                          |
|--------|-------------------------------------|
| Image  | `.jpg` `.jpeg` `.png` `.gif` `.webp` `.avif` |
| Video  | `.mp4` `.webm` `.mov` `.avi` `.mkv` |

## Available commands

| Command      | Description                     |
|--------------|---------------------------------|
| `make run`   | Start the development server    |
| `make build` | Build a binary to `bin/app`     |
| `make tidy`  | Tidy Go module dependencies     |

## Thumbnail cache

Video thumbnails are generated into `cache/thumbnails/`, mirroring the `media/` path structure:

```
cache/
└── thumbnails/
    └── 2026/
        └── 2026-12-25 Skiing in Switzerland/
            └── highlight.jpg   ← generated from highlight.mp4
```

To regenerate all thumbnails, delete the folder contents:

```bash
rm -rf cache/thumbnails && mkdir -p cache/thumbnails
```

The `cache/` directory is excluded from git.

## Project structure

```
.
├── main.go                  Entry point
├── internal/
│   └── handler/
│       └── handler.go       HTTP handlers
├── templates/
│   ├── base.html            Page layout (Bulma + HTMX)
│   ├── index.html           Year listing
│   ├── year.html            Album listing
│   └── album.html           Media grid with lightbox
├── media/                   Source photos and videos (gitignored)
└── cache/
    └── thumbnails/          Generated video thumbnails (gitignored)
```
