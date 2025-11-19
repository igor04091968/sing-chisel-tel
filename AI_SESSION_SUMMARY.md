# AI Session Summary: sing-chisel-tel Project Enhancements

This document summarizes the problems encountered and solutions implemented during an AI-assisted development session for the `sing-chisel-tel` project.

## 1. Tor Binary Integration

**Problem:** The user initially requested integration of the `github.com/opd-ai/go-tor` Go package. This attempt failed due to an invalid version in `go.mod`. The user then clarified they needed a standalone `tor` binary, similar to how `mtg` is handled.

**Solution:**
1.  Reverted all changes related to the `go-tor` Go package.
2.  Downloaded the latest Tor Browser bundle for Linux AMD64 from `torproject.org`.
3.  Extracted the `tor` executable from the bundle.
4.  Moved the `tor` executable to the project root (`/source/sing-chisel-tel/tor`).
5.  Made the `tor` binary executable (`chmod +x tor`).

## 2. Docker Build Issues

**Problem:** Initial Docker build failed with `go: go.mod requires go >= 1.25.1 (running go 1.22.12; GOTOOLCHAIN=local)`. The `golang:1.22-alpine` base image was outdated for the project's Go version requirement.

**Solution:**
1.  Updated the `Dockerfile` to use `golang:1.25-alpine` for the builder stage.
*(Note: This change was later reverted during a `git restore` operation and would need to be re-applied if Docker build is attempted again.)*

## 3. Incorrect Subscription Link Domain

**Problem:** Generated subscription links and client connection links were using the `s-ui` instance's address (e.g., a cloud IP) instead of the desired relay server address (e.g., `gw.iri1968.dpdns.org`), especially when a reverse tunnel was in use.

**Solution:**
1.  **Modified `service/setting.go`:**
    *   Added a new setting key `"subscriptionDomain"` to `defaultValueMap`.
    *   Modified `GetWebDomain()` to first check for `subscriptionDomain`. If set, it uses this value; otherwise, it falls back to the existing `"webDomain"` setting.
    *   Added a `SetSubscriptionDomain(domain string) error` method to update this setting.
2.  **Modified `telegram/bot.go`:**
    *   Updated the `/help` message to include new commands.
    *   Added `case` statements in `handleCommand` for `/set_sub_domain` and `/get_sub_domain`.
    *   Implemented `handleSetSubDomain` and `handleGetSubDomain` functions to interact with the new setting.
*(Note: This feature was initially implemented, then reverted by `git restore`, and then re-implemented.)*

## 4. Missing Chisel Client Logs

**Problem:** `chisel-client`'s internal logs were not visible in `sui`'s output, making debugging difficult.

**Solution:**
1.  **Modified `main.go`:**
    *   Created a custom `io.Writer` called `ChiselLogWriter` that redirects output to `s-ui`'s `logger.Info` (prefixed with `[CHISEL]`).
    *   Set `log.SetOutput(&ChiselLogWriter{})` in `runApp()` after `app.Init()`. This captures all standard `log` package output (which `chisel` uses) and routes it through `s-ui`'s logger.
*(Note: This feature was initially implemented, then reverted by `git restore`, and then re-implemented.)*

## 5. Chisel Client Auto-Start Reliability

**Problem:** `chisel-client` was not reliably auto-starting after `sui` restarts, especially after ungraceful shutdowns (e.g., `^C`). This was due to the `p_id` field in the `chisel_configs` table not being reset to `0`, causing `sui` to incorrectly believe the service was still running.

**Solutions Implemented:**

1.  **Database `p_id` Column Name Correction:**
    *   Identified that GORM maps `PID` in `model.ChiselConfig` to `p_id` in the SQLite database. Corrected database queries to use `p_id`.
2.  **Typo Fix in Default Chisel Config:**
    *   Corrected a typo in `app/app.go` from `"defauilt"` to `"default"` for the default Chisel client config name.
3.  **Graceful Chisel Shutdown in `app.Stop()`:**
    *   **Modified `service/chisel.go`:**
        *   Added `GetActiveChiselConfigIDs()` method to safely retrieve IDs of active services.
        *   Added `StopAllActiveChiselServices()` method to iterate and stop all active services.
        *   **Crucially, modified `StopChisel()` to *always* reset `config.PID` to `0` in the database when a service is stopped, regardless of its prior state.** This ensures proper cleanup of the `p_id` flag.
    *   **Modified `app/app.go`:**
        *   Updated `Stop()` to call `a.chiselService.StopAllActiveChiselServices()` to ensure all running Chisel services are gracefully stopped and their `p_id` flags are reset.
4.  **PID Reset on Application Startup:**
    *   **Modified `app/app.go`'s `Start()` function:**
        *   Added a loop at the beginning of `Start()` to iterate through all `chisel_configs` in the database and explicitly set their `p_id` to `0`. This provides a robust mechanism to ensure all Chisel clients attempt to start on every application launch, even if previous shutdowns were ungraceful.
*(Note: These changes were initially implemented, then reverted by `git restore`, and then re-implemented.)*

## 6. goudp2raw Library Development

**Problem:** The user requested to add `udp2raw`-like functionality to the project, specifically for creating tunnels similar to `udp2raw` and managing QoS parameters. The initial plan was to integrate an external `udp2raw` binary into `s-ui`. However, the user then clarified the need for a *standalone Go library* implementing this functionality.

**Solution:** A new, independent Go library named `goudp2raw` was developed in `/home/igor/gemini_projects/goudp2raw`.

### Key Components and Functionality:
-   **`go.mod`:** Defines the `goudp2raw` module.
-   **`tunnel.go`:** Core interfaces (`Tunnel`, `Server`, `Client`) and `Config` structure for the library.
-   **`raw/conn.go`:** Implements high-level raw socket access using `golang.org/x/net/ipv4`. This layer handles reading and writing IP packets and includes a `SetDSCP` method for QoS marking.
-   **`crypto/cipher.go`:** Provides AES-CBC encryption and decryption with PKCS#7 padding, ensuring secure data transmission.
-   **`transport/icmp.go`:** Implements the ICMP transport layer. It uses the `raw` package for socket operations and the `crypto` package for encryption, encapsulating UDP data within ICMP Echo messages. It also integrates the DSCP setting from the `raw` layer.
-   **`cmd/goudp2raw-server/main.go`:** An example server application that listens for ICMP tunnel traffic, decrypts it, and forwards UDP payloads to a target service. It also handles responses back to the client.
-   **`cmd/goudp2raw-client/main.go`:** An example client application that listens for local UDP traffic, encrypts it, and sends it to the `goudp2raw-server` via the ICMP tunnel. It also receives, decrypts, and forwards responses to the local application.

### Build Process:
-   The library and its example applications are built using `go mod tidy` to manage dependencies and `CGO_ENABLED=0 go build` to create static binaries, avoiding Cgo-related linking issues.

### QoS/DSCP Implementation:
-   The `goudp2raw` library now supports setting DSCP values (0-63) via a command-line flag (`-dscp`) in both the client and server applications. This allows for marking outgoing tunnel packets for Quality of Service prioritization by network infrastructure.

## 7. goudp2raw Library Integration into s-ui

The `goudp2raw` library has been integrated into the `s-ui` project as a local dependency.

### Integration Steps:
-   **Project Relocation:** The `goudp2raw` project was moved into the `sing-chisel-tel` directory at `/home/igor/gemini_projects/sing-chisel-tel/goudp2raw`.
-   **`go.mod` Update:** The `sing-chisel-tel/go.mod` file was updated with a `replace goudp2raw => ./goudp2raw` directive to allow `s-ui` to import the local `goudp2raw` module.
-   **`model/udp2raw.go` Update:** The `Udp2rawConfig` struct in `sing-chisel-tel/model/udp2raw.go` was updated to include the `DSCP int` field and change the `Args` field to `json.RawMessage`, aligning it with the `goudp2raw.Config` structure and `s-ui`'s database requirements.
-   **`service/udp2raw.go` Creation:** A new `Udp2rawService` was created in `sing-chisel-tel/service/udp2raw.go`. This service manages the lifecycle of `goudp2raw` tunnels, including CRUD operations on `Udp2rawConfig`, starting/stopping `goudp2raw` processes, and handling auto-start/shutdown.
-   **`util/scanner.go` Creation:** A helper utility `NewLineScanner` was created in `sing-chisel-tel/util/scanner.go` to facilitate logging process output from `goudp2raw` executables.
-   **`app/app.go` Integration:**
    -   The `App` struct now includes a `udp2rawService` field.
    -   The `udp2rawService` is initialized in `app.Init()`.
    -   `ResetPIDs()` and `StartAllUdp2rawServices()` are called in `app.Start()` for proper tunnel management on application startup.
    -   `StopAllActiveUdp2rawServices()` is called in `app.Stop()` for graceful shutdown.
    -   A getter `GetUdp2rawService()` is provided.
-   **`api/apiV2Handler.go` Integration:**
    -   The `APIv2Handler` struct now includes a `udp2rawAPI` field.
    -   `udp2rawAPI` is initialized in `NewAPIv2Handler`.
    -   Its routes are registered in `initRouter`.
-   **`api/udp2raw.go` (Planned):** This file will define the API endpoints (`/api/v2/udp2raw`) for managing `goudp2raw` configurations via the web panel. It will include handlers for `GET`, `POST`, `PUT`, `DELETE`, `start`, and `stop` operations, interacting with `app.GetUdp2rawService()`.
-   **Telegram Bot Integration (Planned):** New Telegram bot commands will be added to manage `goudp2raw` tunnels.

## Current Status

All identified issues regarding Tor binary integration, subscription link generation, Chisel logging, and Chisel client auto-start/shutdown reliability have been addressed. The `goudp2raw` library has been successfully developed and integrated into the `s-ui` project's backend, with API and Telegram bot integration planned for the next steps.
