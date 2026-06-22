# Photo & Video Album

A simple local photo and video album viewer. Designed for **Linux and macOS only** — not supported on Windows.

## Requirements

- [Go](https://go.dev/dl/) 1.22 or later
- [ffmpeg](https://ffmpeg.org/download.html) — for video thumbnail generation
- `make`

Install dependencies if not already present:

```bash
# Ubuntu / Debian
sudo apt install make ffmpeg
sudo snap install go --classic

# macOS
brew install go ffmpeg
```

Verify the installation:

```bash
ffmpeg -version
```

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

## Write mode (photo rotation)

Write mode lets you rotate photos directly from the browser. It is protected by a secret key that only you know.

### Configuring the secret key

**Option 1 — inline when running (temporary, lost when the terminal closes):**

```bash
WRITE_KEY=mysecret make run
```

**Option 2 — export in your shell profile (persists across sessions):**

Add this line to `~/.bashrc` or `~/.zshrc`:

```bash
export WRITE_KEY=mysecret
```

Then reload the profile and start the server normally:

```bash
source ~/.bashrc   # or source ~/.zshrc
make run
```

**Option 3 — `.env` file with a helper script:**

Create a `.env` file in the project root (it is gitignored):

```bash
echo "WRITE_KEY=mysecret" > .env
```

Then source it before running:

```bash
source .env && make run
```

> Choose a key that is hard to guess — treat it like a password.

### Activating write mode

Once the server is running with `WRITE_KEY` set, append `?key=<your-key>` to any URL in the browser — you only need to do this once:

```
http://localhost:8080/?key=mysecret
```

The server validates the key and sets a session cookie, so write mode stays active as you navigate between albums without repeating the query param.

### Using write mode

While write mode is active:
- A yellow banner is shown on album pages as a reminder
- Each photo card displays ↶ (counter-clockwise) and ↷ (clockwise) rotate buttons
- Clicking a button rotates the actual file on disk 90° and refreshes the thumbnail in place — no page reload needed
- Videos do not have rotate buttons

Write mode is disabled entirely when `WRITE_KEY` is not set.

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
