# Copilot / AI Agent Instructions

This file helps AI coding agents be productive in the **s-ui** repository. Focus on concrete, discoverable patterns and commands.

## Overview
- **Purpose:** Single Go-based binary (`sui`) with Vue frontend for managing server configurations, with integrated Telegram bot for remote administration.
- **Major components:**
  - `api/` — HTTP handlers (Gin framework) 
  - `service/` — Business logic layer (ChiselService, GostService, MTProtoService, GreService, TapService, etc.)
  - `core/` — Sing-box integration
  - `database/` — SQLite persistence (GORM models in `database/model/`)
  - `web/` — Static frontend serving
  - `sub/` — Subscription link generation
  - `telegram/` — Bot commands and handlers
  - `app/` — Application lifecycle (Init → Start → Stop)
  - `frontend/` — Vue 3 + TypeScript UI

## Key Files to Read First
- `main.go` — Entry point; routes to either `app.NewApp()` or CLI via `cmd/`
- `app/app.go` — App lifecycle, service initialization, auto-start of tunnels/services
- `config/config.go` — Environment-driven config (env vars `SUI_DB_FOLDER`, `SUI_LOG_LEVEL`, `SUI_DEBUG`)
- `api/apiHandler.go` — Central HTTP routing: `POST/:postAction`, `GET/:getAction` → service method calls
- `database/db.go` — DB initialization with AutoMigrate for all models including `GostConfig`
- `telegram/bot.go` — Telegram bot command routing, `AppServices` interface definition
- `service/gost.go` — **Embedded GOST reverse tunnel implementation** (no external binaries)

## Build & Run
- **Full build** (includes frontend): `./build.sh` (compiles frontend → `web/html/`, then Go backend)
- **Backend only**: `go build -ldflags "-w -s" -tags "with_quic,with_grpc,with_utls,with_acme,with_gvisor" -o sui main.go`
- **Run dev frontend**: `cd frontend && npm install && npm run dev` (Vite on localhost:5173)
- **Docker**: `docker-compose up --build`

## Environment Variables & Config Files
- `SUI_DB_FOLDER` — Override DB location (defaults to binary directory + `/db`)
- `SUI_LOG_LEVEL` — `debug`, `info`, `warn`, `error`
- `SUI_DEBUG` — Set to `true` for debug logging
- `telegram_config.json` — Optional; if present and `Enabled: true`, Telegram bot starts
- Example: See `telegram_config.json.example`

## Project-Specific Conventions & Patterns

### Service Architecture Pattern
1. **Model** in `database/model/` (e.g., `GostConfig`, `ChiselConfig`) — GORM struct with `gorm.Model`
2. **Service** in `service/` (e.g., `GostService`, `ChiselService`) — business logic, lifecycle management
3. **Telegram Handlers** in `telegram/bot.go` (e.g., `handleAddGost*`, `handleStartGost`) — parse args, call service methods, send messages
4. **API Handlers** in `api/` — HTTP endpoints that also call service methods
5. **Auto-startup** — App auto-starts enabled tunnels on `app.Start()` (see `app/app.go`)

**Example:** Adding a new tunnel service:
- Create model in `database/model/mytunnel.go` with fields (Name, ListenPort, ServerAddress, Status, etc.)
- Add to `database/db.go` AutoMigrate list
- Create service in `service/mytunnel.go` with `NewMyTunnelService()`, `Start()`, `Stop()`, `GetAll()`, `Create()`, `Delete()`
- Expose via `app.GetMyTunnelService()` in `app/app.go`
- Add to `AppServices` interface in `telegram/bot.go` 
- Add command handlers in `telegram/bot.go` (e.g., `/add_mytunnel`)

### HTTP API Pattern
- Routes are action-based: `POST /api/:postAction` and `GET /api/:getAction`
- Example: `POST /api/save` with JSON body containing object type, action, and data
- Service method is called by action name (string switch in `ApiService.Save`)
- Responses are JSON with `code`, `msg`, `data` fields (see `api/utils.go`)

### Telegram Bot Pattern
- Receive message → `handler()` → check admin ID → parse command
- Command switch-case in `handleCommand()` calls specific handler functions
- Handler functions take `(ctx context.Context, b *bot.Bot, message *models.Message, args []string)`
- Extract args, validate, call service, send response via `b.SendMessage()`
- If command fails, send error message immediately

### GOST Reverse Tunnel (Embedded, No External Binary)
- **Location:** `service/gost.go` (fully self-contained)
- **Technology:** Go standard `net` library + `io.Copy` for bidirectional forwarding
- **Lifecycle:** `StartGost()` creates TCP listener, starts background goroutine, updates DB status
- **Forwarding:** `forwardConnection()` connects to target, copies traffic both directions, handles timeout
- **Shutdown:** `StopGost()` cancels context, closes listener, updates DB status
- **Persistence:** All configs in `GostConfig` table; tunnels auto-restart on app startup if status was "up"
- **Telegram Commands:**
  - `/add_gost_client <name> <listen:port> <target:port>` → creates + starts
  - `/list_gost` → shows all with status
  - `/start_gost <name>`, `/stop_gost <name>`, `/remove_gost <name>`
- **Extending:** Modify `forwardConnection()` to add encryption, compression, or protocol-specific logic while keeping service interface stable

### Auto-Start Behavior
- In `app.Start()`: App fetches all `GostConfig` records where `Status == "up"`
- For each enabled tunnel, calls `gostService.StartGost(config)`
- Similarly for Chisel, MTProto, GRE, TAP services
- **Important:** When modifying Start/Stop logic, ensure DB status stays consistent with actual runtime state

### Logging & Diagnostics
- Use `logger` helpers (not `log` package directly)
- `logger.Info()`, `logger.Warning()`, `logger.Error()` for different levels
- Logs are stored in-memory and exposed via `/api/logs` endpoint
- Accessible from Telegram bot via `/logs` command

## Critical Integration Points
- **Chisel:** Embedded via `github.com/jpillora/chisel/client` and `/server` packages
- **Sing-box Core:** Managed in `core/` package; interacts with `service.ConfigService`
- **MTProto:** External `mtg` binary (via `service.MTProtoService` exec wrapper) — manage process lifecycle
- **GRE/TAP:** System-level via CLI commands (require root); managed via `service.GreService`, `service.TapService`
- **Frontend-Backend:** REST API via `api/apiHandler.go`; frontend consumes via Axios in `src/` (see `src/api.ts` pattern)

## Quick Examples (Copyable)
```bash
# Build backend only (dev)
go build -o sui main.go

# Build with all tags (production)
go build -ldflags "-w -s" -tags "with_quic,with_grpc,with_utls,with_acme,with_gvisor" -o sui main.go

# Run locally
./sui

# Start reverse tunnel via Telegram
/add_gost_client my-tunnel 0.0.0.0:9999 192.168.1.100:22
```

## What to Avoid
- **Don't change DB path logic in `config/config.go`** without testing Windows/Linux fallbacks
- **Don't modify Telegram bot without updating help text** — keep `/help` command synchronized with actual commands
- **Don't add UI elements without adding types in `frontend/src/types/`** — TypeScript compilation will fail
- **Don't store PID as actual OS process ID** — use as status flag (0 = stopped, non-zero = running) like Chisel does
- **Don't start services without updating both `app.Start()` and `app.Stop()`** for proper lifecycle
- **Don't forget to add new models to `database/db.go` AutoMigrate** or DB won't have tables

## Testing & Debugging
- **Enable debug logging:** Set env var `SUI_DEBUG=true`
- **Check logs:** GET `/api/logs?limit=100&level=debug`
- **Test service directly:** Create small Go script that imports and calls `service.GostService` methods
- **Telegram bot test:** Use `/help` to verify all commands are registered
- **Frontend dev:** `npm run dev` in `frontend/` folder for HMR + TypeScript checking

## If Something Is Missing
- Ask about specific runtime scenarios: local dev, Docker, production deployment on server
- Point to failing command output — I will inspect related scripts (`build.sh`, `docker-compose.yml`, relevant service files)
- Ask about specific model fields or service method behavior; I'll read the source and explain

---
**End of instructions — ask for clarification on architecture, patterns, or specific components if needed.**
