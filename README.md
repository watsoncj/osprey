# osprey

A browser history forensics tool for macOS, Linux, and Windows. It discovers history databases, decodes URLs, flags concerning content by category, and detects possible incognito browsing activity.

## Architecture

Osprey uses a persistent agent model for monitoring endpoints:

- **Agent** (`cmd/agent`): runs as a daemon on monitored machines, performs periodic browser scans, and uploads JSON reports to the collection server.
- **Server** (`cmd/server`): HTTP server that receives and stores reports from agents.
- **CLI** (`cmd/osprey`): standalone tool for local one-off scans.

## Supported Browsers

- Google Chrome
- Microsoft Edge
- Brave
- Firefox
- Safari (macOS only)

## Features

- **Auto-discovery** — finds history databases for all supported browsers across user profiles
- **URL decoding** — extracts search queries from Google, Bing, and DuckDuckGo; resolves YouTube video titles via the oEmbed API
- **Content flagging** — scans URLs, titles, and decoded data against keyword lists organized by category (violence, self-harm, bullying, drugs, weapons, hate speech, adult content)
- **Incognito detection** — cross-references Chromium Favicons databases against History to surface URLs visited in private/incognito mode
- **Persistent monitoring** — agent runs as a system service, uploading reports on a configurable interval
- **No CGO** — uses [modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite) for easy cross-compilation

## Installation

Requires Go 1.25+.

```sh
# Build all components
make

# Build individually
make osprey     # CLI tool
make agent      # Agent (native platform)
make server     # Collection server

# Cross-compile agent for Windows
make agent-windows
```

## Usage

### Local Scan (CLI)

```sh
# Scan all browsers, last 24 hours (default)
./osprey

# Scan the last 2 weeks, output JSON
./osprey -lookback 2w -format json

# Scan specific database files
./osprey /path/to/History /path/to/places.sqlite
```

### Collection Server

```sh
# Start the server (default: listen on :8080, store in ./data)
./osprey-server

# Custom settings
./osprey-server -listen :9090 -data /var/lib/osprey

# Install as a system service
sudo ./osprey-server -install -listen :8080 -data /var/lib/osprey

# Uninstall the service
sudo ./osprey-server -uninstall
```

**API Endpoints:**

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/reports` | Receive a report from an agent |
| `GET` | `/api/reports` | List all reports (filter: `?hostname=X`) |
| `GET` | `/api/reports/{hostname}/{id}` | Retrieve a specific report |

### Agent Deployment

```sh
# One-shot scan (prints report to stdout)
./osprey-agent

# Run as daemon, uploading to server every hour
./osprey-agent -server http://server:8080 -interval 1h

# Install as a system service (runs on boot)
sudo ./osprey-agent -install -server http://server:8080 -interval 1h

# Override hostname identifier
sudo ./osprey-agent -install -server http://server:8080 -hostname workstation-42

# Uninstall the service
sudo ./osprey-agent -uninstall
```

### Agent Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-lookback` | `24h` | How far back to analyze (`24h`, `5d`, `2w`) |
| `-format` | `text` | Output format: `text` or `json` |
| `-server` | | Server URL (enables daemon mode) |
| `-interval` | `1h` | Scan interval in daemon mode |
| `-hostname` | `os.Hostname()` | Override machine identity |
| `-install` | `false` | Install as a system service |
| `-uninstall` | `false` | Remove the system service |

### CLI Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-lookback` | `24h` | How far back to analyze (`24h`, `5d`, `2w`) |
| `-format` | `text` | Output format: `text` or `json` |

## Report Output

The text report includes:

- **Summary** — total and flagged visit counts per database
- **Top domains** — most visited domains (top 10)
- **Flags by category** — counts of flagged visits grouped by category
- **YouTube videos** — resolved titles and video IDs with timestamps
- **Flagged items** — detailed listing of each flagged visit with URL, title, decoded data, and matched keywords
- **Incognito indicators** — URLs found in Favicons DB but absent from History
- **Decoded URLs** — unflagged URLs with extracted structured data (search queries, etc.)

## Project Structure

```
cmd/
  osprey/              CLI tool (local scanning)
  agent/               Agent binary (daemon mode + one-shot)
  server/              Collection server
internal/
  app/                 Core pipeline: config, orchestration
  browsers/            Browser adapters (Chrome, Edge, Brave, Firefox, Safari)
  decoder/             URL decoders (Google, Bing, DuckDuckGo, YouTube)
  finder/              Database file discovery and glob expansion
  flagging/            Keyword-based content flagging engine
  model/               Shared data types (Visit, Flag, RunReport, etc.)
  report/              Output formatters (text, JSON)
  store/               File-based report storage
  upload/              HTTP upload client for agent → server
  sqliteio/            Read-only SQLite helpers
```
