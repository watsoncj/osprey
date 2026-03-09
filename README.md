# osprey

A browser history forensics tool for macOS, Linux, and Windows. It discovers history databases, decodes URLs, flags concerning content by category, and detects possible incognito browsing activity.

## Architecture

Osprey uses a persistent agent model for monitoring endpoints:

- **Agent** (`cmd/agent`): runs as a daemon on monitored machines, performs periodic browser scans, and uploads raw visit data to the collection server.
- **Server** (`cmd/server`): HTTP server that receives raw visit data from agents, handles URL decoding, content flagging, deduplication, and aggregation. Serves both a JSON API and a server-rendered web dashboard.

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
- **Persistent monitoring** — agent runs as a system service, uploading visit data on a configurable interval
- **Server-side processing** — decoding, flagging, and deduplication happen on the server, keeping the agent lightweight
- **No CGO** — uses [modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite) for easy cross-compilation

## Installation

Requires Go 1.25+.

```sh
# Build all components
make

# Build individually
make agent      # Agent (native platform)
make server     # Collection server

# Cross-compile agent + server for Windows
make win64

# Remove built binaries
make clean
```

## Usage

### Collection Server

```sh
# Start the server (default: listen on :9753, store in ./data)
./osprey-server

# Custom settings
./osprey-server -listen :9090 -data /var/lib/osprey

# Install as a system service
sudo ./osprey-server -install -listen :9753 -data /var/lib/osprey

# Uninstall the service
sudo ./osprey-server -uninstall
```

**API Endpoints:**

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/visits` | Receive raw visit data from an agent |
| `GET` | `/api/visits` | Query visits (filter: `?hostname=`, `?flagged=`, `?browser=`, `?since=`, `?until=`, `?limit=`, `?offset=`) |
| `GET` | `/api/hosts` | Per-host summary stats (total visits, flagged count, latest visit time) |
| `POST` | `/api/reports` | Receive a report from an agent (legacy) |

### Web UI

The server serves a web dashboard alongside the API on the same port:

- **Dashboard** (`/`) — per-host visit stats: hostname, total visits, flagged visits, latest visit time, top flagged category
- **Host detail** (`/hosts/{hostname}`) — paginated visit timeline with filter controls (flagged-only, browser, date range)
- **Flagged visits** (`/flagged`) — fleet-wide view of all flagged visits across all hosts, grouped by category
- **Incognito indicators** — per-host incognito indicators accessible from the host detail page, with decoded data for each indicator

### Agent Deployment

The agent requires a server URL and runs as a daemon, uploading raw visit data on a configurable interval.

```sh
# Run as daemon, uploading to server every hour
./osprey-agent -server http://server:9753 -interval 1h

# Install as a system service (runs on boot)
sudo ./osprey-agent -install -server http://server:9753 -interval 1h

# Override hostname identifier
sudo ./osprey-agent -install -server http://server:9753 -hostname workstation-42

# Uninstall the service
sudo ./osprey-agent -uninstall
```

### Agent Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-lookback` | `24h` | How far back to analyze (`24h`, `5d`, `2w`) |
| `-server` | | Server URL (required) |
| `-interval` | `1h` | Scan interval |
| `-hostname` | `os.Hostname()` | Override machine identity |
| `-install` | `false` | Install as a system service |
| `-uninstall` | `false` | Remove the system service |

## Project Structure

```
cmd/
  agent/               Agent binary (daemon mode)
  server/              Collection server + web UI
internal/
  app/                 Core pipeline: config, orchestration
  browsers/            Browser adapters (Chrome, Edge, Brave, Firefox, Safari)
  decoder/             URL decoders (Google, Bing, DuckDuckGo, YouTube)
  finder/              Database file discovery and glob expansion
  flagging/            Keyword-based content flagging engine
  ingest/              Server-side processing pipeline (decode, flag, dedup)
  model/               Shared data types (Visit, Submission, Flag, etc.)
  spool/               Agent-side spool for failed uploads
  sqliteio/            Read-only SQLite helpers
  store/               Visit and incognito indicator storage
  upload/              HTTP upload client for agent → server
  web/                 Server-rendered HTML dashboard (templates + embedded assets)
```
