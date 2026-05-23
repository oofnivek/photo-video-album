# Photo & Video Album

A simple local photo and video album viewer. Drop media files into the `media/` folder and browse them in your browser.

## Requirements

- [Go](https://go.dev/dl/) 1.22 or later

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

Organise files into albums by creating named subfolders inside `media/`. Folder names can be a year, a date, or a descriptive title:

```
media/
├── 2025/
│   ├── photo1.jpg
│   └── clip.mp4
├── 2026/
│   └── photo2.png
└── 2026-12-25 Skiing in Switzerland/
    ├── family.jpg
    └── highlight.mp4
```

Each subfolder becomes an album on the home page. Refresh after adding files — no server restart needed.
    
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

## Project structure

```
.
├── main.go                  Entry point
├── internal/
│   └── handler/
│       └── handler.go       HTTP handlers
├── templates/
│   ├── base.html            Page layout (Bulma + HTMX)
│   └── index.html           Media grid
└── media/                   Put your photos and videos here
```
