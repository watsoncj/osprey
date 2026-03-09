# Architecture: Persistent Agent + Collection Server

## Overview

Shift from on-demand remote scanning to a persistent agent model:

- **Agent** (`cmd/agent`): runs continuously on client Windows machines, performs periodic scans, uploads JSON reports to the server over HTTP.
- **Server** (`cmd/server`): HTTP endpoint that receives and stores reports from agents.
- **CLI** (`cmd/osprey`): retains local scan ability; adds commands to view collected reports from the server store.

The existing scan pipeline (`internal/app`, `internal/browsers`, `internal/decoder`, `internal/flagging`) is unchanged — the agent just runs it on a loop and POSTs results instead of printing to stdout.

---

## Tasks

### Phase 1: Agent Daemon Mode

- [x] **1.1 — Add daemon mode to agent CLI**
  Add `-server` and `-interval` flags to `cmd/agent/main.go`. When `-server` is set, the agent runs in a loop: scan → upload → sleep. When not set, current one-shot behavior is preserved.
  - `-server http://host:port` — server URL to upload reports to
  - `-interval 1h` — how often to scan (reuse `app.Duration`)
  - `-hostname` — optional override for machine identity (defaults to `os.Hostname()`)
  
- [x] **1.2 — agent CLI should be able to install itself as a system service**

- [x] **1.3 — Add upload client (`internal/upload`)**
  New package with a single function: `Upload(ctx, serverURL, hostname string, report model.RunReport) error`. POSTs the report as JSON to `POST /api/reports` with a `X-Hostname` header. Returns error on non-2xx.

- [x] **1.4 — Add `Hostname` field to `model.RunReport`**
  Add `Hostname string \`json:"hostname"\`` to `RunReport` so reports are tagged with their source machine.

- [x] **1.5 — Remove agent embedding and scp/remote execution feature**
  Runtime model has changed from scp of agent on demand to an installation of daemon on monitored endpoints.

### Phase 2: Collection Server

- [x] **2.1 — Create server binary (`cmd/server/main.go`)**
  Minimal HTTP server. Flags: `-listen :8080`, `-data ./data` (directory to store reports). Endpoints:
  - `POST /api/reports` — receive and store a report
  - `GET /api/reports` — list stored reports (optional query: `?hostname=X`)
  - `GET /api/reports/{id}` — retrieve a single report

- [x] **2.2 — Report storage (`internal/store`)**
  File-based storage. Each report saved as `{data_dir}/{hostname}/{timestamp}.json`. Package provides:
  - `Save(hostname string, report model.RunReport) error`
  - `List(hostname string) ([]ReportMeta, error)`
  - `Load(hostname, id string) (model.RunReport, error)`

- [x] **2.3 — server binary should be able to self install as a system service**

### Phase 3: Integration & Build

- [x] **3.1 — Update Makefile**
  Add `server` target. Update `agent` target to build the daemon-capable agent. Keep existing `controller` target working for local/remote use.

- [x] **3.2 — Update README.md**
  Document the new persistent agent deployment model: how to install the agent as a Windows scheduled task or service, server setup, viewing reports.

### Phase 4: Polish (future)

- [x] **4.1 — Agent retry on upload failure**
  If upload fails, save report to a local spool directory and retry on next cycle.

- [x] **4.2 — Server authentication**
  Add a shared secret / API key for upload endpoint.

- [x] **4.3 — Windows service support**
  Make the agent installable as a Windows service (e.g., via `golang.org/x/sys/windows/svc`).

- [x] **4.4 — TLS for server**
  Support `-cert` and `-key` flags on the server for HTTPS.

- [x] **4.5 — Add GHA builder that hosts releases on the GitHub Releases page**

### Phase 5: Web Interface

Server-side rendered HTML UI using `html/template` + `embed`. No JS frameworks, no build step. Serves alongside the existing JSON API on the same port.

- [x] **5.1 — Embedded template + static asset scaffold (`internal/web`)**
  New package. Use `//go:embed` to bundle templates and a CSS file. Provide a `Handler(s *store.Store) http.Handler` that returns a mux for all web routes. Base layout template with nav header (site title, link to dashboard).

- [x] **5.2 — Dashboard page (`GET /`)**
  Lists all known hostnames. For each host show: hostname, number of reports, latest report timestamp, latest flagged-visit count. Each row links to the host detail page. Empty state when no reports exist.

- [x] **5.3 — Host detail page (`GET /hosts/{hostname}`)**
  Lists all reports for a hostname, newest first. Each row shows: timestamp, per-DB total visits, per-DB flagged visits. Each row links to the report detail page. Breadcrumb back to dashboard.

- [x] **5.4 — Report detail page (`GET /reports/{hostname}/{id}`)**
  Full report view, mirroring the sections in `internal/report/text.go`:
  - Scan metadata (hostname, started at, cutoff)
  - Per-DB sections: summary stats, top domains table, category flag counts
  - YouTube videos list (timestamp, title, video ID)
  - Flagged items detail (URL, title, decoded data, flag labels)
  - Incognito indicators list
  - Decoded URLs (unflagged) list
  Breadcrumbs back to host → dashboard.

- [x] **5.5 — Wire web routes into `cmd/server/main.go`**
  Mount `web.Handler(store)` on the server mux. Web routes (`/`, `/hosts/...`, `/reports/...`) coexist with API routes (`/api/...`). No new flags needed.

- [x] **5.6 — Styling and polish**
  Minimal embedded CSS: clean table layout, responsive, color-coded flag categories (reuse category names as CSS classes), zebra-striped rows. Dark/light friendly neutral palette. No external CDN dependencies.

### Phase 6: Visit-Centric Architecture

Move from report-centric storage to a visit-centric model. The agent sends raw visit data; the server handles decoding, flagging, deduplication, and aggregation. The web UI shifts from browsing timestamped report snapshots to browsing a unified, deduplicated visit timeline per host.

#### 6.1 — New data model (`internal/model`)

- [x] **6.1.1 — Add `Submission` type**
  Lightweight payload the agent sends. Fields: `Hostname`, `ScannedAt time.Time`, `Visits []RawVisit`, `IncognitoIndicators []RawIncognitoIndicator`. No decoded/flag data — just raw observations.

- [x] **6.1.2 — Add `RawVisit` type**
  Minimal visit record: `Time`, `URL`, `Title`, `Browser`, `DBPath`. No `Decoded`, no `Flags`.

- [x] **6.1.3 — Add `RawIncognitoIndicator` type**
  Same as current `IncognitoIndicator` but without `Decoded`: just `URL`, `Browser`, `DBPath`.

- [x] **6.1.4 — Keep existing `Visit`, `DecodedURL`, `Flag`, `RunReport` types**
  These become server-side-only types used for storage and rendering. `Visit` retains `Decoded` and `Flags` fields — they're populated by the server after ingestion.

#### 6.2 — Server-side processing pipeline (`internal/ingest`)

- [x] **6.2.1 — Create `internal/ingest` package**
  New package with `Process(sub model.Submission) []model.Visit`. Takes a submission, runs each raw visit through the decoder registry and flagger, returns enriched visits. Reuses `internal/decoder` and `internal/flagging` unchanged.

- [x] **6.2.2 — Process incognito indicators**
  `ProcessIncognito(indicators []model.RawIncognitoIndicator) []model.IncognitoIndicator` — runs decoder on each URL to produce the full `IncognitoIndicator` with `Decoded` populated.

#### 6.3 — Visit-centric storage (`internal/store`)

- [x] **6.3.1 — Add visit store**
  New storage methods alongside the existing report store. File-based: `{data_dir}/{hostname}/visits.jsonl` — one JSON object per line (append-only). Each line is an enriched `Visit` (with `Decoded` and `Flags` populated by the server).

- [x] **6.3.2 — Deduplication on append**
  Before appending, deduplicate against existing visits. Key: `(URL, Time, Browser)`. Load existing visits into a set, skip duplicates. This handles the agent re-submitting overlapping windows.

- [x] **6.3.3 — Incognito indicator storage**
  Store incognito indicators in `{data_dir}/{hostname}/incognito.jsonl`. Same append-only JSONL, dedup by `(URL, Browser)`.

- [x] **6.3.4 — Query methods**
  - `ListHosts() []string` — enumerate hostname directories
  - `LoadVisits(hostname string, opts VisitQuery) ([]Visit, error)` — load visits with optional filters (time range, flagged-only, browser, limit/offset)
  - `LoadIncognito(hostname string) ([]IncognitoIndicator, error)`
  - `HostStats(hostname string) (HostStats, error)` — aggregate stats: total visits, flagged count, latest visit time, category counts, top domains

#### 6.4 — Agent changes

- [x] **6.4.1 — Agent sends `Submission` instead of `RunReport`**
  Simplify agent's `runDaemon`: scan DBs → collect raw visits + incognito indicators → build `Submission` → POST to server. Remove decoder/flagging imports from agent. The `app.Run` function gets a `RunRaw` variant (or `Run` is modified) that skips decode/flag steps and returns raw visits.

- [x] **6.4.2 — Track last-scan timestamp**
  After a successful upload, persist the latest visit timestamp to `{spool_dir}/last_scan.json`. On next cycle, use this as the cutoff floor (max of configured lookback and last-scan time) to avoid re-submitting old history. Best-effort — if the file is missing, fall back to the configured lookback.

- [x] **6.4.3 — Update upload client**
  `Upload` sends a `Submission` to `POST /api/visits` instead of `POST /api/reports`. Update spool to store `Submission` instead of `RunReport`.

- [x] **6.4.4 — Remove one-shot mode and `cmd/osprey` binary**
  Delete `cmd/osprey/` directory and `internal/report/` package. Remove one-shot `runOnce` from agent — `-server` becomes required. Remove the `osprey` target from Makefile and GHA release workflow. Update README.

#### 6.5 — Server API changes (`cmd/server`)

- [x] **6.5.1 — Add `POST /api/visits` endpoint**
  Receives a `Submission`, runs it through `ingest.Process`, deduplicates, and appends to the visit store. Returns `201` with count of new visits inserted.

- [x] **6.5.2 — Add `GET /api/visits` endpoint**
  Query visits across hosts. Query params: `hostname`, `flagged` (bool), `browser`, `since`, `until`, `limit`, `offset`. Returns JSON array of enriched visits.

- [x] **6.5.3 — Add `GET /api/hosts` endpoint**
  Returns per-host summary stats (total visits, flagged count, latest visit time, top categories).

- [x] **6.5.4 — Keep legacy `POST /api/reports` working (deprecated)**
  Existing report upload endpoint continues to work for backward compatibility with old agents. Internally converts the `RunReport` into a `Submission` and feeds it through the new pipeline. Log a deprecation notice.

#### 6.6 — Web UI overhaul (`internal/web`)

- [x] **6.6.1 — New dashboard**
  Replace report-count-centric dashboard with visit-centric view. Per-host row shows: hostname, total visits, flagged visits, latest visit time, top flagged category. Sort by most-recent activity.

- [x] **6.6.2 — Host detail page — visit timeline**
  Replace list-of-reports with a paginated visit timeline. Shows all visits for a host, newest first. Each row: timestamp, URL (truncated), title, browser, flag badges. Filter controls: flagged-only toggle, browser dropdown, date range.

- [x] **6.6.3 — Visit detail view**
  Clicking a visit row expands or navigates to show: full URL, title, all decoded data, all flags with category/keyword/source, browser, DB path.

- [x] **6.6.4 — Flagged visits summary page**
  New page: `/flagged` — shows all flagged visits across all hosts, grouped by category. Quick overview of concerning activity fleet-wide.

- [x] **6.6.5 — Incognito indicators page**
  Per-host incognito indicators view, accessible from the host detail page. Shows decoded data for each indicator.

- [x] **6.6.6 — Remove report detail page**
  Once the visit-centric pages are live, remove the report detail page and the old report-list host page. Keep the legacy API endpoint but drop the web views.

#### 6.7 — Cleanup

- [x] **6.7.1 — Final cleanup**
  Remove any dead code left over from the report-centric model. Ensure `go vet ./...` and `go build ./...` pass cleanly.

- [x] **6.7.2 — Update README.md**
  Document the new architecture: agents send raw visits, server processes them, visit-centric dashboard.

- [x] **6.7.3 — Update Makefile if needed**
  Ensure build targets still work with any new/removed packages.
