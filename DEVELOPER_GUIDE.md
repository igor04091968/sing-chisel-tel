# s-ui Developer Guide

This guide provides a comprehensive overview of the s-ui application, with a focus on the integrated Telegram bot and the development workflow.

## 1. Project Overview

**s-ui** is a web-based user interface for managing server configurations. A key feature of this project is the integrated **Telegram bot**, which allows administrators to manage the application remotely through a simple command-line interface.

The bot provides a convenient way to perform administrative tasks without needing to log in to the web UI.

## 2. Architecture

The Telegram bot is integrated into the main 's-ui' application as a separate module. The interaction between the bot and the main application is designed to be loosely coupled, which is achieved through the 'AppServices' interface.

### The 'AppServices' Interface

The 'AppServices' interface (defined in 'telegram/bot.go') acts as a "contract" between the bot and the main application. It exposes a set of methods that the bot can use to interact with the application's core services, such as:

-   'RestartApp()'
-   'GetConfigService()'
-   'GetUserByEmail(email string)'
-   And many more...

This design has several advantages:
-   **Decoupling:** The bot doesn't need to know about the internal implementation of the application's services.
-   **Testability:** It allows for easier testing of the bot in isolation by providing a mock implementation of the 'AppServices' interface.
-   **Maintainability:** Changes to the core application are less likely to break the bot, as long as the 'AppServices' contract is maintained.

### Initialization

The bot is initialized and started as a goroutine in the 'app.Start()' function (in 'app/app.go'). The bot's configuration is read from 'telegram_config.json'.

## 2.1 Chisel Service Refactoring

The Chisel service has been refactored to embed the `jpillora/chisel` library directly into the `s-ui` application, rather than relying on an external `chisel` executable. This provides more robust control over the Chisel server/client lifecycle and improves error handling.

**Key Changes:**

*   **In-process Execution**: Chisel servers and clients are now run as Go goroutines within the `s-ui` application.
*   **Lifecycle Management**: The `service.ChiselService` now manages the lifecycle of these in-process Chisel instances using `context.WithCancel` for graceful shutdown.
*   **PID Field Repurposed**: The `PID` field in the `model.ChiselConfig` database entry is no longer an operating system process ID. It now serves as a simple flag (`1` for running, `0` for stopped) to indicate the service's intended state. The logic for updating this PID has been refined to ensure it accurately reflects the running status of the Chisel client/server, preventing false positives in status checks.
*   **Telegram Bot Command Updates**: The Telegram bot commands (`/add_chisel_server`, `/add_chisel_client`, `/start_chisel`, `/stop_chisel`, `/list_chisel`, `/remove_chisel`, `/delete_all_chisel`) have been updated to reflect these changes. OS-level process checks have been removed, and the status is now derived from the internal state managed by `service.ChiselService`.
*   **Improved Error Logging**: The `StartChisel` function now captures and logs `stderr` output from the Chisel library, providing better diagnostics if a Chisel service fails to start or exits prematurely.
*   **Database-driven Configuration & Auto-start**: Chisel client configurations are now stored in the `s-ui` database. On application startup, `s-ui` checks for existing Chisel client configurations and automatically starts any that are not running. If no Chisel client configuration exists, a default placeholder (`default-chisel-client`) is created in the database.
*   **Chisel Client Args Parsing Fix**: The parsing of the `Args` field for Chisel clients has been corrected to properly extract authentication arguments (`--auth`) and TLS flags (`--tls`) from the remote strings, and assign them to the appropriate fields in `chclient.Config`, resolving issues with malformed remote errors.

**Impact on Development:**

*   Developers no longer need to ensure the `chisel` executable is present in the system's `$PATH`.
*   Debugging Chisel-related issues can now leverage `s-ui`'s internal logging.
*   Chisel client configurations are now dynamic and manageable via the web panel and Telegram bot.

## 2.2 Chisel Web Panel Integration

The Chisel service management has been integrated into the web panel, allowing users to add, edit, and manage Chisel server and client configurations directly from the UI.

**Key Changes:**

*   **Backend API**: New API endpoints have been exposed to perform CRUD operations on Chisel configurations, leveraging the existing `service.ChiselService`.
    *   The `ApiService.Save` method now handles `chisel` objects.
    *   The `ApiService.LoadData` and `ApiService.LoadPartialData` methods now include Chisel configurations.
*   **Frontend UI**:
    *   A new `CHISEL` service type has been added to `frontend/src/types/services.ts`.
    *   A dedicated Vue component (`frontend/src/components/services/Chisel.vue`) has been created for configuring Chisel services.
    *   The `frontend/src/layouts/modals/Service.vue` component now dynamically renders the `Chisel.vue` component when a Chisel service is being added or edited.
*   **Type System Enhancements**:
    *   The `listen_port` property in `frontend/src/types/inbounds.ts` (within the `Listen` interface) has been made optional to accommodate services that may not require a listening port (e.g., Chisel clients).
    *   Type casting and imports were adjusted in frontend Vue components to ensure TypeScript compatibility.

**Impact on Development:**

*   Developers can now extend the web panel to manage other services by following a similar pattern of backend API exposure and frontend component integration.

## 2.3 GOST Reverse Tunnel Service (Embedded)

GOST tunnels have been fully integrated as an **embedded service** within the `s-ui` application. No external binaries are required.

**Key Implementation:**

*   **Service File:** `service/gost.go` - Implements `GostService` with full lifecycle management.
*   **Database Model:** `database/model/gost.go` - `GostConfig` struct stores tunnel configurations persistently.
*   **Telegram Integration:** GOST commands in `telegram/bot.go` provide `/add_gost_*`, `/list_gost`, `/start_gost`, `/stop_gost`, `/remove_gost`.
*   **Technology:** Uses Go standard library `net` package for TCP listening and `io.Copy` for bidirectional forwarding.
*   **Concurrency:** Each tunnel runs in its own goroutine with `context.CancelFunc` for graceful shutdown.
*   **No External Deps:** Zero external binary dependencies - everything compiled into the `sui` executable.

**Architecture Pattern:**

The GOST service follows the same architectural pattern as Chisel:
1. **Tunnel Config** → Stored in `GostConfig` model
2. **Service Layer** → `GostService` manages lifecycle (Start/Stop)
3. **Background Goroutine** → Accepts connections and forwards to target
4. **Context Cancellation** → Graceful shutdown on Stop
5. **Database Persistence** → Tunnels survive app restart

**Extending GOST Functionality:**

To add more advanced features (encryption, compression, protocol-specific handling):
- Modify the `forwardConnection()` function in `service/gost.go`
- Example: Insert encryption layer between `clientConn` and `targetConn` reads/writes
- Maintain the same service interface for compatibility with Telegram bot

## 3.1 Telegram Bot Commands for Advanced Services

The Telegram bot has been enhanced with new commands to manage MTProto Proxy, GRE Tunnels, and TAP Tunnels.

### MTProto Proxy Commands

These commands allow you to manage MTProto proxies, which are run as external `mtg` processes.

*   `/add_mtproto <name> <port> <secret> [ad_tag]`
    *   **Description:** Creates and starts a new MTProto proxy.
    *   **Parameters:**
        *   `<name>`: Unique name for the proxy.
        *   `<port>`: Port on which the proxy will listen.
        *   `<secret>`: 32-character hexadecimal MTProto secret.
        *   `[ad_tag]` (optional): Tag for advertising Telegram channels.
    *   **Example:** `/add_mtproto myproxy 443 aabbccddeeff11223344556677889900 mychannel`
*   `/list_mtproto`
    *   **Description:** Lists all configured MTProto proxies and their status.
*   `/remove_mtproto <name>`
    *   **Description:** Stops and removes an MTProto proxy by name.
    *   **Parameters:**
        *   `<name>`: Name of the proxy to remove.
    *   **Example:** `/remove_mtproto myproxy`
*   `/start_mtproto <name>`
    *   **Description:** Starts a stopped MTProto proxy by name.
    *   **Parameters:**
        *   `<name>`: Name of the proxy to start.
    *   **Example:** `/start_mtproto myproxy`
*   `/stop_mtproto <name>`
    *   **Description:** Stops a running MTProto proxy by name.
    *   **Parameters:**
        *   `<name>`: Name of the proxy to stop.
    *   **Example:** `/stop_mtproto myproxy`
*   `/gen_mtproto_secret`
    *   **Description:** Generates a new 32-character hexadecimal MTProto secret.

### GRE Tunnel Commands

These commands allow you to manage kernel-level GRE tunnels. `s-ui` must be run with `root` privileges for these to work.

*   `/add_gre <name> <local_ip> <remote_ip> <tunnel_address> [interface_name]`
    *   **Description:** Creates and starts a new GRE tunnel.
    *   **Parameters:**
        *   `<name>`: Unique name for the tunnel.
        *   `<local_ip>`: Local physical IP address.
        *   `<remote_ip>`: Remote physical IP address.
        *   `<tunnel_address>`: IP address and mask for the tunnel itself (e.g., "10.0.0.1/30").
        *   `[interface_name]` (optional): Name of the network interface to create (e.g., `gre0`). If omitted, one will be generated.
    *   **Example:** `/add_gre gretun0 192.168.1.100 203.0.113.5 10.0.0.1/30`
*   `/list_gre`
    *   **Description:** Lists all configured GRE tunnels and their status.
*   `/remove_gre <name>`
    *   **Description:** Removes a GRE tunnel by name.
    *   **Parameters:**
        *   `<name>`: Name of the tunnel to remove.
    *   **Example:** `/remove_gre gretun0`
*   `/start_gre <name>`
    *   **Description:** Starts a stopped GRE tunnel by name.
    *   **Parameters:**
        *   `<name>`: Name of the tunnel to start.
    *   **Example:** `/start_gre gretun0`
*   `/stop_gre <name>`
    *   **Description:** Stops a running GRE tunnel by name.
    *   **Parameters:**
        *   `<name>`: Name of the tunnel to stop.
    *   **Example:** `/stop_gre gretun0`

### TAP Tunnel Commands

These commands allow you to manage TAP interfaces. `s-ui` must be run with `root` privileges for these to work.

*   `/add_tap <name> <ip_address> [mtu] [interface_name]`
    *   **Description:** Creates and starts a new TAP interface.
    *   **Parameters:**
        *   `<name>`: Unique name for the TAP interface.
        *   `<ip_address>`: IP address and mask for the TAP interface (e.g., "192.168.50.1/24").
        *   `[mtu]` (optional): Maximum Transmission Unit (MTU) for the interface. Defaults to 1500.
        *   `[interface_name]` (optional): Name of the network interface to create (e.g., `tap0`). If omitted, one will be generated.
    *   **Example:** `/add_tap tap0 192.168.50.1/24 1420`
*   `/list_tap`
    *   **Description:** Lists all configured TAP interfaces and their status.
*   `/remove_tap <name>`
    *   **Description:** Removes a TAP interface by name.
    *   **Parameters:**
        *   `<name>`: Name of the TAP interface to remove.
    *   **Example:** `/remove_tap tap0`
*   `/start_tap <name>`
    *   **Description:** Starts a stopped TAP interface by name.
    *   **Parameters:**
        *   `<name>`: Name of the TAP interface to start.
    *   **Example:** `/start_tap tap0`
*   `/stop_tap <name>`
    *   **Description:** Stops a running TAP interface by name.
    *   **Parameters:**
        *   `<name>`: Name of the TAP interface to stop.
    *   **Example:** `/stop_tap tap0`

### GOST Reverse Tunnels

These commands allow you to manage embedded reverse TCP tunnels using GOST. Unlike MTProto/GRE/TAP which rely on external processes, GOST tunnels are **fully embedded** in the `s-ui` application with no external binaries required.

**Architecture:**
- GOST tunnels use Go's standard `net` library for TCP listening and connection forwarding.
- Bidirectional traffic forwarding via `io.Copy` for reliable data transfer.
- Built-in context cancellation for graceful shutdown.
- No root privileges required for basic TCP tunneling.

**Commands:**

*   `/add_gost_server <name> <listen_port> [extra_args]`
    *   **Description:** Creates and starts a new GOST server tunnel (local listening socket).
    *   **Parameters:**
        *   `<name>`: Unique name for the tunnel.
        *   `<listen_port>`: Port on which the tunnel will listen (0.0.0.0:<port>).
        *   `[extra_args]` (optional): Additional arguments or target specification (e.g., "192.168.1.100:22").
    *   **Example:** `/add_gost_server reverse-ssh 9999`
    *   **Use case:** Listen on port 9999 and wait for manual target specification.

*   `/add_gost_client <name> <server:port> <target_host:target_port> [extra_args]`
    *   **Description:** Creates and starts a new GOST client tunnel (connects and forwards to target).
    *   **Parameters:**
        *   `<name>`: Unique name for the tunnel.
        *   `<server:port>`: Listening address and port (format: "0.0.0.0:9999").
        *   `<target_host:target_port>`: Target to forward connections to (format: "192.168.1.100:22").
        *   `[extra_args]` (optional): Reserved for future use.
    *   **Example:** `/add_gost_client ssh-reverse 0.0.0.0:9999 192.168.1.100:22`
    *   **Use case:** Listen on 0.0.0.0:9999, forward all incoming connections to 192.168.1.100:22 (reverse SSH tunnel).

*   `/list_gost`
    *   **Description:** Lists all configured GOST tunnels, their status, and target addresses.
    *   **Example:** `/list_gost`

*   `/remove_gost <name>`
    *   **Description:** Stops and removes a GOST tunnel configuration.
    *   **Parameters:**
        *   `<name>`: Name of the tunnel to remove.
    *   **Example:** `/remove_gost ssh-reverse`

*   `/start_gost <name>`
    *   **Description:** Starts a stopped GOST tunnel.
    *   **Parameters:**
        *   `<name>`: Name of the tunnel to start.
    *   **Example:** `/start_gost ssh-reverse`

*   `/stop_gost <name>`
    *   **Description:** Stops a running GOST tunnel.
    *   **Parameters:**
        *   `<name>`: Name of the tunnel to stop.
    *   **Example:** `/stop_gost ssh-reverse`

**Implementation Details:**

- **Database Model:** `database/model/gost.go` defines `GostConfig` with fields for name, mode (server/client), listen/server addresses, ports, and status.
- **Service Layer:** `service/gost.go` implements `GostService` with methods:
  - `StartGost(cfg *model.GostConfig)`: Creates TCP listener and starts forwarding goroutine.
  - `StopGost(id uint)`: Cancels context and closes listener.
  - `GetAllGostConfigs()`, `GetGostConfigByName()`, `CreateGostConfig()`, `DeleteGostConfig()`.
- **Connection Forwarding:** `forwardConnection()` helper function handles bidirectional traffic copy with proper error handling.
- **Database Integration:** GOST configs are persisted in the SQLite database and survive application restarts.

**Developer Notes:**

*   All GOST functionality is **embedded** - no external binaries or system dependencies (beyond TCP networking).
*   The tunnel status is tracked in the database (`status` field: "up"/"down").
*   Each tunnel runs in its own goroutine with independent context cancellation.
*   Connections are forwarded with a 5-second dial timeout to prevent hanging on unreachable targets.
*   Modify `service/gost.go` to add custom forwarding logic (e.g., protocol-specific handling, encryption, compression).

## 3.2 UDP Tunnel (udp2raw) - Pure Go Implementation

As per user request, a new pure Go implementation of a `udp2raw`-like service has been added. This service is designed to encapsulate UDP traffic into other protocols for obfuscation and prioritization.

### 3.2.1 Core Architecture & Features

-   **Pure Go:** The entire implementation is written in Go and compiled into the main `sui` binary. It does **not** require any external `udp2raw` C++ binaries.
-   **Low-Level Packet Crafting:** The service uses raw IP sockets (`net.IPConn`) to send manually crafted packets. This allows for full control over the IP and transport layer headers.
-   **DSCP Marking for QoS:** The primary feature is the ability to set DSCP (Differentiated Services Code Point) values on outgoing packets for traffic prioritization. This is achieved by setting the `IP_TOS` socket option using `syscall.SetsockoptInt`. The DSCP value is configurable for each tunnel.
-   **FakeTCP Mode (MVP):** The initial implementation is a Minimum Viable Product (MVP) that supports a client-side **FakeTCP** mode. It wraps UDP payloads into TCP segments with the `SYN` flag set to mimic a connection attempt.
-   **Privilege Requirement:** Due to the use of raw sockets (`net.ListenIP`) and setting socket options, the `sui` application **must be run with `root` privileges** or have the `CAP_NET_RAW` capability (`sudo setcap cap_net_raw=+ep /path/to/sui`).

### 3.2.2 How It Works (FakeTCP Client Mode)

1.  **Local UDP Listener:** The service listens on a local UDP port (e.g., `127.0.0.1:1080`).
2.  **Packet Interception:** An application (e.g., a SOCKS5 proxy) sends its UDP traffic to this local port.
3.  **Raw Socket Creation:** The service creates a raw IP socket for sending data to the remote server.
4.  **DSCP & Socket Options:** It uses `SyscallConn().Control()` on the raw socket to set the `IP_HDRINCL` option (telling the kernel we will provide the IP header) and the `IP_TOS` option (to set the DSCP value from the tunnel's configuration).
5.  **Packet Crafting (`gopacket`):** For each incoming UDP datagram, the service:
    a.  Extracts the UDP payload.
    b.  Constructs a new packet with an `IPv4` header and a `TCP` header.
    c.  The `SYN` flag is set on the TCP header to create the "FakeTCP" packet.
    d.  The UDP payload is placed as the TCP payload.
    e.  Checksums are calculated.
6.  **Packet Sending:** The newly crafted packet is sent to the destination via the raw socket.

### 3.2.3 Telegram Bot Commands

The following commands are available to manage UDP tunnels:

*   `/add_udptunnel <name> <mode> <listen_port> <remote_addr:port> [dscp]`
    *   **Description:** Creates and starts a new UDP tunnel.
    *   **Parameters:**
        *   `<name>`: Unique name for the tunnel.
        *   `<mode>`: Tunneling mode (currently only `faketcp` is implemented).
        *   `<listen_port>`: The local UDP port to listen on.
        *   `<remote_addr:port>`: The remote server address.
        *   `[dscp]` (optional): A DSCP value (0-63). E.g., `46` for Expedited Forwarding (EF).
    *   **Example:** `/add_udptunnel mytunnel faketcp 1080 8.8.8.8:443 46`
*   `/list_udptunnels`: Lists all configured UDP tunnels.
*   `/remove_udptunnel <name>`: Stops and removes a tunnel.
*   `/start_udptunnel <name>`: Starts a stopped tunnel.
*   `/stop_udptunnel <name>`: Stops a running tunnel.

### 3.2.4 Limitations & Future Work

-   The current implementation is a proof-of-concept and only supports the client-side of FakeTCP mode.
-   Server-side logic, ICMP mode, and raw UDP mode are not yet implemented.
-   Packet fields like source port and sequence numbers are currently hardcoded and should be randomized for production use.

## 4. The "Hot Restart" Mechanism

The '/restart' command triggers a "hot restart" of the application's services. This is a crucial feature for applying configuration changes without downtime.

### The Problem with a Simple Restart

A naive implementation of a restart would be to simply stop and start the entire application process. However, this caused a "Conflict" error with the Telegram API: 'Conflict: terminated by other getUpdates request'.

This error occurs because the old bot instance doesn't have enough time to gracefully close its connection to the Telegram servers before the new instance tries to connect.

### The Solution: A True Hot Restart

The current implementation solves this problem by performing a true "hot restart" that **does not restart the Telegram bot itself**.

Here's how it works:
1.  When the '/restart' command is received, the 'app.RestartApp()' function is called.
2.  'app.Stop()' is called, but it **no longer stops the Telegram bot**. It only stops the other application services (web server, cron jobs, etc.).
3.  'app.Init()' is called to re-initialize all the services.
4.  'app.Start()' is called to start the services again. Crucially, 'app.Start()' now includes logic to **prevent the Telegram bot from being started if it's already running**.

This approach ensures that there is only ever one instance of the bot running, thus avoiding the "Conflict" error. The running bot seamlessly starts interacting with the newly restarted services.

## 5. Development Workflow

### Building the Application

The 'build.sh' script in the root directory is used to build the entire application. It performs two main steps:
1.  **Builds the frontend:** It runs 'npm install' and 'npm run build' in the 'frontend' directory.
2.  **Builds the backend:** It runs 'go build' to create the 'sui' executable in the root directory.

### Running for Development

To run the application for development, use the 'runSUI.sh' script. This script sets the necessary environment variables and starts the 'sui' executable.

Alternatively, you can run the executable directly:
'''bash
SUI_DB_FOLDER="db" SUI_DEBUG=true ./sui
'''

## 6. Troubleshooting

### "Conflict" Error on Restart

-   **Problem:** The Telegram bot fails to get updates with a "Conflict" error.
-   **Cause:** Multiple instances of the bot are running simultaneously.
-   **Solution:** The hot-restart mechanism was re-designed to ensure only one instance of the bot is running at all times. See the "Hot Restart" section for details.

### 'unexpected end of JSON input'

-   **Problem:** Occurred when adding a new user.
-   **Cause:** The 'Links' field in the 'model.Client' struct was not being initialized, resulting in invalid JSON.
-   **Solution:** The 'Links' field is now initialized with an empty JSON array ('json.RawMessage("[]")').

### 'converting NULL to string is unsupported'



-   **Problem:** Occurred when adding a new user.

--   **Cause:** A SQL query was returning 'NULL' for a JSON field, and the database driver could not convert 'NULL' to a string.

-   **Solution:** The SQL query was modified to use 'COALESCE' to replace 'NULL' values with an empty string.





## 7. Recent Fixes and Improvements (Nov 2025)



This section details recent bug fixes and enhancements to the application.



### 7.1 Chisel Service Management



A number of issues related to Chisel service management have been addressed.



*   **Web UI Save Logic:** The backend logic for saving Chisel configurations from the web interface was implemented. Previously, attempting to save a Chisel service would result in an `unknown object chisel` error. This was resolved by:

    1.  Adding a `Save(tx *gorm.DB, act string, data json.RawMessage) error` method to `service/chisel.go` to handle create, update, and delete operations within a database transaction.

    2.  Adding a `case "chisel":` to the `switch` statement in `service/config.go` to delegate save operations to the new `ChiselService.Save` method.



*   **Argument Parsing:** The argument parsing for Chisel clients in `service/chisel.go` has been made more robust. It now correctly handles `--auth`, `--tls-skip-verify`, and `--tls` flags, preventing them from being misinterpreted as remote addresses, which previously caused `Failed to decode remote` errors.



*   **Default Configuration:** The default Chisel client configuration created on first startup (in `app/app.go`) has been improved. It now includes the `--tls-skip-verify` flag and an example remote mapping (`R:8000:localhost:8080`) to provide a better out-of-the-box experience for users with TLS-enabled servers. The default name was also shortened from `default-chisel-client` to `defauilt`.



### 7.2 Database Auto-Migration



The application startup process has been improved to handle database migrations more gracefully.



*   **Missing Tables:** The `services` and `tokens` tables were not being created on startup for users with older database files, leading to `no such table` errors. This was fixed by adding `&model.Service{}` and `&model.Tokens{}` to the `db.AutoMigrate` call in `database/db.go`.



### 7.3 Telegram Bot Commands



*   **/list_chisel Command:** This command was failing silently for some users. The issue was traced to potential Markdown parsing errors in the Telegram API if configuration names or arguments contained special characters. The fix was to remove Markdown formatting from the response in `telegram/bot.go`, ensuring the message is always sent as plain text.

### 7.4 Architectural Fixes

*   **API Service Initialization:** A major architectural flaw was discovered where the `api.ApiService` struct was not being initialized, leading to a `nil` pointer panic when its methods were called. This was the root cause of the `500 Internal Server Error` on all API endpoints. The issue was resolved by refactoring the `service.ChiselService` to be stateless (similar to `service.SettingService`), removing its dependency on an initialized `db` field and instead using the global `database.GetDB()` getter. This is a pragmatic fix to prevent the panic, though a more thorough dependency injection refactor could be considered in the future.

### 7.5 Chisel TLS Connection

*   **WebSocket Handshake Error:** A `websocket: bad handshake` error was occurring when connecting to a Chisel server with TLS enabled. Server-side logs revealed the root cause: `client sent an HTTP request to an HTTPS server`. This was happening because the client was connecting using the `ws://` protocol instead of the secure `wss://` protocol. The fix was to prepend `https://` to the server address in `service/chisel.go` when TLS is enabled, which correctly sets the WebSocket scheme to `wss://`.

*   **SNI Support:** To further improve TLS compatibility, especially with servers hosting multiple domains, the `ServerName` field in the `chclient.TLSConfig` is now set to the Chisel server's address. This ensures the correct certificate is presented during the TLS handshake.

## 9. GRE Tunneling

The `s-ui` application now supports Generic Routing Encapsulation (GRE) tunneling, allowing for the creation and management of GRE tunnel interfaces directly from the application's API.

### 9.1. Core Architecture

-   **Kernel-Level Integration:** GRE tunnels are created and managed by interacting with the Linux kernel's networking stack via the `vishvananda/netlink` Go library.
-   **Database-Driven Configuration:** GRE tunnel configurations are stored in the `s-ui` SQLite database (`gre_tunnels` table).
-   **Centralized Control:** The `service.GreService` manages the lifecycle of GRE tunnel interfaces.
-   **Privilege Requirement:** Creating and configuring GRE tunnel interfaces requires `CAP_NET_ADMIN` privileges. Therefore, the `s-ui` application must be run with `root` privileges for this functionality to work.

### 9.2. Network Capabilities

-   **Interface Creation:** Dynamically create virtual GRE tunnel interfaces.
-   **IP Configuration:** Assign local and remote IP addresses to the GRE tunnel.
-   **Status Management:** Bring the tunnel interface up or down.

### 9.3. How It Works: From API to Tunnel

1.  **User Action:** A user creates a GRE tunnel configuration via the API.
2.  **API Call:** The frontend sends a request to `POST /api/v2/gre`.
3.  **Service Layer:** The `api.GreAPI` calls `service.GreService.CreateGreTunnel`.
4.  **Database Interaction:** The configuration is saved to the database.
5.  **Netlink Interaction:** `service.GreService` uses `netlink` to:
    a.  Add the GRE tunnel interface (`netlink.LinkAdd`).
    b.  Assign an IP address (`netlink.AddrAdd`).
    c.  Bring the interface up (`netlink.LinkSetUp`).
6.  **Tunnel Established:** The GRE tunnel interface is created and configured in the operating system.

## 10. TAP Tunneling

The `s-ui` application now supports TAP tunneling, allowing for the creation and management of virtual TAP network interfaces.

### 10.1. Core Architecture

-   **User-Space Device Creation:** TAP devices are created using the `songgao/water` Go library, which interacts with `/dev/net/tun` (or equivalent).
-   **Kernel-Level Configuration:** After creation, the TAP device is configured (assigning IP, MTU, bringing up) by interacting with the Linux kernel's networking stack via the `vishvananda/netlink` Go library.
-   **Database-Driven Configuration:** TAP tunnel configurations are stored in the `s-ui` SQLite database (`tap_tunnels` table).
-   **Centralized Control:** The `service.TapService` manages the lifecycle of TAP interfaces.
-   **Privilege Requirement:** While creating the TAP device itself might not always require `root` (depending on `/dev/net/tun` permissions), its full configuration (IP address, MTU, bringing up) requires `CAP_NET_ADMIN` privileges. Therefore, the `s-ui` application must be run with `root` privileges for this functionality to work.

### 10.2. Network Capabilities

-   **Interface Creation:** Dynamically create virtual TAP interfaces.
-   **IP Configuration:** Assign IP addresses to the TAP interface.
-   **MTU Configuration:** Set the Maximum Transmission Unit for the TAP interface.
-   **Status Management:** Bring the TAP interface up or down.
-   **Raw Ethernet Frame Access:** Once created and configured, the TAP interface allows `s-ui` (or another user-space program) to read and write raw Ethernet frames.

### 10.3. How It Works: From API to Interface

1.  **User Action:** A user creates a TAP tunnel configuration via the API.
2.  **API Call:** The frontend sends a request to `POST /api/v2/tap`.
3.  **Service Layer:** The `api.TapAPI` calls `service.TapService.CreateTapTunnel`.
4.  **Database Interaction:** The configuration is saved to the database.
5.  **Water/Netlink Interaction:** `service.TapService` uses `songgao/water` to create the TAP device and then `netlink` to:
    a.  Get the netlink link for the created device.
    b.  Set the MTU (`netlink.LinkSetMTU`).
    c.  Assign an IP address (`netlink.AddrAdd`).
    d.  Bring the interface up (`netlink.LinkSetUp`).
6.  **TAP Interface Established:** The TAP interface is created and configured in the operating system.

## 11. MTProto Proxy

The `s-ui` application now supports managing MTProto Proxies for Telegram, allowing users to bypass censorship and access the messaging service. This is implemented by controlling an external `mtg` binary.

### 11.1. Core Architecture

-   **External Process Management:** The `s-ui` application manages the `mtg` (MTProto Proxy) binary as an external process using Go's `os/exec` package.
-   **Database-Driven Configuration:** MTProto Proxy configurations are stored in the `s-ui` SQLite database (`mtproto_proxy_configs` table).
-   **Centralized Control:** The `service.MTProtoService` manages the lifecycle of `mtg` processes (start, stop).
-   **Privilege Requirement:** While `mtg` itself might not require `root` to run (depending on the listen port), `s-ui`'s ability to manage external processes and potentially bind to privileged ports (like 443) might necessitate running `s-ui` with elevated privileges or proper `setcap` configuration for the `sui` binary.

### 11.2. Network Capabilities

-   **MTProto Protocol Support:** Provides a proxy for Telegram's proprietary MTProto protocol.
-   **Censorship Circumvention:** Designed to bypass network restrictions and DPI by mimicking legitimate Telegram traffic.
-   **Obfuscation:** Supports MTProto's built-in obfuscation mechanisms.
-   **Configurable Secret:** Allows setting a unique secret for each proxy instance.
-   **AdTag Support:** Supports optional `AdTag` for promoting Telegram channels.

### 11.3. How It Works: From API to Proxy

1.  **User Action:** A user creates or starts an MTProto Proxy configuration via the API.
2.  **API Call:** The frontend sends a request to `POST /api/v2/mtproto` or `POST /api/v2/mtproto/:id/start`.
3.  **Service Layer:** The `api.MTProtoAPI` calls `service.MTProtoService.StartMTProtoProxy`.
4.  **Database Interaction:** The configuration is saved to the database.
5.  **External Process Launch:** `service.MTProtoService` constructs command-line arguments (e.g., `--bind-to`, `--secret`, `--ad-tag`) and launches the `mtg` binary using `os/exec.CommandContext`.
6.  **Proxy Running:** The `mtg` binary starts listening on the specified port, handling MTProto traffic. `s-ui` monitors its lifecycle and updates its status in the database.

## 12. Privilege Requirements

For the full functionality of GRE, TAP, and MTProto Proxy management, the `s-ui` application requires elevated privileges.

-   **GRE and TAP Tunneling:** Creating and configuring kernel-level network interfaces (GRE) and fully configuring TAP interfaces (assigning IP, MTU, bringing up) requires `CAP_NET_ADMIN` capability.
-   **MTProto Proxy (External Process):** While the `mtg` binary itself might not always need `root` (e.g., if listening on a non-privileged port > 1024), `s-ui`'s ability to manage external processes and potentially bind `mtg` to privileged ports (like 443) might necessitate running `s-ui` with elevated privileges or proper `setcap` configuration for the `sui` binary.

**Recommendation:** It is recommended to run the `sui` executable with `root` privileges or configure appropriate Linux capabilities (e.g., `sudo setcap cap_net_admin,cap_net_bind_service=+ep /path/to/sui`) if fine-grained control is desired.