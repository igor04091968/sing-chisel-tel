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
*   **PID Field Repurposed**: The `PID` field in the `model.ChiselConfig` database entry is no longer an operating system process ID. It now serves as a simple flag (`1` for running, `0` for stopped) to indicate the service's intended state.
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

## 3. Bot Commands

The bot supports a variety of commands for managing the 's-ui' application.

| Command                               | Description                                                                                             |
| ------------------------------------- | ------------------------------------------------------------------------------------------------------- |
| '/start'                              | Displays a welcome message.                                                                             |
| '/help'                               | Lists all available commands.                                                                           |
| '/adduser <email> <traffic_gb> [tag]' | Adds a new user with a specified traffic limit. An optional inbound tag can be provided.                |
| '/deluser <email>'                    | Deletes a user.                                                                                         |
| '/stats'                              | Shows online statistics.                                                                                |
| '/logs'                               | Displays application logs.                                                                              |
| '/restart'                            | Performs a "hot restart" of the application's services.                                                 |
| '/sublink <email>'                    | Generates subscription links for a user.                                                                |
| '/add_in <type> <tag> <port>'         | Creates a new inbound interface.                                                                        |
| '/add_out <json>'                     | Creates a new outbound interface (requires a full JSON configuration).                                  |
| '/list_users'                         | Lists all users.                                                                                        |
| '/setup_service'                      | Provides instructions for setting up 's-ui' as a 'systemd' service.                                     |
| '/help_devel'                         | Displays a detailed help message for developers, including troubleshooting information.                 |

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


