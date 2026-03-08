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
