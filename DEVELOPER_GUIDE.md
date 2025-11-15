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
*   **Telegram Bot Command Updates**: The Telegram bot commands (`/add_chisel_server`, `/add_chisel_client`, `/start_chisel`, `/stop_chisel`, `/list_chisel`, `/remove_chisel`) have been updated to reflect these changes. OS-level process checks have been removed, and the status is now derived from the internal state managed by `service.ChiselService`.
*   **Improved Error Logging**: The `StartChisel` function now captures and logs `stderr` output from the Chisel library, providing better diagnostics if a Chisel service fails to start or exits prematurely.

**Impact on Development:**

*   Developers no longer need to ensure the `chisel` executable is present in the system's `$PATH`.
*   Debugging Chisel-related issues can now leverage `s-ui`'s internal logging.

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
