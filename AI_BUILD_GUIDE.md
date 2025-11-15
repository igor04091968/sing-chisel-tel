# AI Build and Push Guide for S-UI Project

This guide provides step-by-step instructions for an AI agent to build the S-UI project, configure Git, push the project to a GitHub repository, and trigger a release workflow via GitHub Actions.

## 1. Project Acquisition and Setup

**Goal:** Obtain the S-UI project source code.

**Instructions:**
1.  **Clone the repository:** If the project is not already present, clone it from its source. (For this scenario, assume the project is already in the working directory `/source/s-ui/`).
2.  **Navigate to the project root:** Ensure the current working directory is `/source/s-ui/`.

## 2. Dependency Installation and Project Build

**Goal:** Install all necessary dependencies and compile the S-UI application.

**Instructions:**
1.  **Build the project:** Execute the `build.sh` script located in the project root. This script handles both frontend (Node.js) and backend (Go) compilation.
    ```bash
    ./build.sh
    ```
    *   **Expected Outcome:** Successful compilation of both frontend and backend. The `sui` executable should be created in the project root.
    *   **Troubleshooting (Node.js Setup):** If `actions/setup-node` fails with "The operation was canceled", try changing the Node.js version in `.github/workflows/release.yml` from `22` to `20`.
        *   **Action:** Modify `.github/workflows/release.yml`:
            ```yaml
            # ...
                  - name: Setup Node.js
                    uses: actions/setup-node@v5
                    with:
                      node-version: '20' # Changed from '22'
                      registry-url: 'https://registry.npmjs.org'
            # ...
            ```
        *   **Re-commit and Re-push:** After modification, commit the change and push to GitHub.
    *   **Troubleshooting (Frontend Build Errors - TypeScript)**: If the frontend build fails with TypeScript errors after modifying frontend components or types (e.g., `Property 'mode' is missing`, `Type 'number | undefined' is not assignable to type 'number'`, `Cannot find name 'CHISEL'`).
        *   **Action 1 (Optional `listen_port`):** Ensure `listen_port` is optional in `frontend/src/types/inbounds.ts` if it's not always required.
            ```typescript
            // In frontend/src/types/inbounds.ts
            export interface Listen {
              listen: string
              listen_port?: number // Changed from 'listen_port: number'
              // ...
            }
            ```
        *   **Action 2 (Undefined check):** Add null/undefined checks for optional properties before accessing them.
            ```typescript
            // Example in frontend/src/layouts/modals/Inbound.vue
            // if (this.inbound.listen_port > 65535 || this.inbound.listen_port < 1) return false
            if (this.inbound.listen_port === undefined || this.inbound.listen_port > 65535 || this.inbound.listen_port < 1) return false
            ```
        *   **Action 3 (Import Types):** Ensure all necessary types (e.g., `CHISEL`) are imported into the script section of Vue components where they are used for type casting or type inference.
            ```typescript
            // Example in frontend/src/layouts/modals/Service.vue
            import { SrvTypes, createSrv, CHISEL } from '@/types/services' // Add CHISEL here
            ```

## 3. Git Repository Setup and Push

**Goal:** Initialize a Git repository, commit the project, and push it to a specified GitHub remote.

**Instructions:**
1.  **Initialize Git:** If not already a Git repository, initialize one in the project root.
    ```bash
    git init
    ```
2.  **Configure Git (Branch Name):** Set the default branch name to `main`.
    ```bash
    git branch -M main
    ```
3.  **Address .gitignore issues (if any):** Ensure all necessary project files, especially the `frontend/` directory, are not ignored by Git.
    *   **Troubleshooting (Missing `frontend/`):** If the `frontend/` directory is not pushed to GitHub, check `.gitignore` for `frontend/` entry.
        *   **Action:** Remove `frontend/` line from `.gitignore`.
            ```bash
            # Example of removing the line
            # Use a text editor or 'sed' command to remove the line
            # sed -i '/frontend\//d' .gitignore
            ```
        *   **Re-add and Re-commit:** After modifying `.gitignore`, proceed to add and commit.
4.  **Add all project files:** Stage all files for commit.
    ```bash
    git add .
    ```
5.  **Commit changes:** Create an initial commit.
    ```bash
    git commit -m "Initial commit of S-UI project"
    ```
6.  **Add GitHub Remote:** Add the remote GitHub repository. **The user must provide the full GitHub repository URL (e.g., `https://github.com/username/repo-name.git`).**
    ```bash
    git remote add origin <GITHUB_REPOSITORY_URL>
    ```
    *   **Example:** `git remote add origin https://github.com/igor04091968/sing-chisel-tel.git`
7.  **Push to GitHub:** Push the committed changes to the remote repository.
    ```bash
    git push -u origin main
    ```
    *   **Troubleshooting (Authentication Failure):** If `git push` fails due to authentication, the Git environment is not configured for non-interactive authentication.
        *   **Action (User Intervention Required):** The user must configure SSH keys on their GitHub account and ensure the local Git environment uses SSH for the remote, or configure a Git credential helper for HTTPS. An AI cannot perform interactive authentication.

## 4. Triggering GitHub Actions Release Workflow

**Goal:** Trigger the `release.yml` workflow on GitHub to compile the project and generate release artifacts.

**Instructions:**
1.  **Create a Git Tag:** Create a version tag for the release (e.g., `alfa-1.0.0`).
    ```bash
    git tag alfa-1.0.0
    ```
2.  **Push the Tag:** Push the created tag to GitHub. This action will trigger the `release.yml` workflow.
    ```bash
    git push origin alfa-1.0.0
    ```
3.  **Monitor Workflow Status:** Navigate to the "Actions" tab of the GitHub repository in a web browser to monitor the workflow run.
    *   **URL Example:** `https://github.com/igor04091968/sing-chisel-tel/actions`
    *   **Expected Outcome:** The "Release S-UI" workflow should run and complete successfully, producing build artifacts.
    *   **Troubleshooting (Workflow Failures):**
        *   **"No such file or directory: frontend"**: This indicates the `frontend/` directory was not pushed. Refer to **Step 3.3** for resolution.
        *   **"Error: The operation was canceled" during Node.js setup**: Refer to **Step 2.1** for resolution.
        *   **"Username and password required" for Docker login**: This indicates missing Docker Hub credentials.
            *   **Action (User Intervention Required):** The user must add `DOCKER_HUB_USERNAME` and `DOCKER_HUB_TOKEN` as repository secrets in GitHub settings. Alternatively, if Docker images are not desired, the `docker.yml` workflow can be removed.
                *   **To remove `docker.yml`:**
                    ```bash
                    rm .github/workflows/docker.yml
                    git add .github/workflows/docker.yml
                    git commit -m "Remove docker.yml workflow"
                    git push origin main
                    ```
                *   After removing, create and push a new tag to re-trigger the release workflow.

## 5. Accessing Release Artifacts

**Goal:** Download the compiled project artifacts.

**Instructions:**
1.  **Navigate to GitHub Actions:** In a web browser, go to the "Actions" tab of the repository.
2.  **Select Workflow Run:** Click on the successful "Release S-UI" workflow run.
3.  **Download Artifacts:** Scroll down to the "Artifacts" section. The compiled `tar.gz` files for various platforms will be available for download.

## 6. Chisel Client Configuration and Management

The `s-ui` application now includes enhanced functionality for managing Chisel client configurations:

*   **Database-driven Configuration**: Chisel client configurations are stored in the application's database.
*   **Automatic Startup**: On `s-ui` startup, the application automatically checks for and starts any configured Chisel clients.
*   **Default Configuration**: If no Chisel client configuration exists in the database, a default placeholder named `default-chisel-client` is automatically created. This allows for easy initial setup and subsequent modification via the web panel or Telegram bot.
*   **Telegram Bot Command**: A new Telegram bot command, `/delete_all_chisel`, has been added. This command allows for the complete removal of all Chisel server and client configurations from the database, including stopping any running instances.
*   **Chisel Client Args Parsing**: The internal parsing logic for Chisel client arguments (`Args` field) has been improved to correctly extract authentication arguments (`--auth`) and TLS flags (`--tls`) from the remote strings, and assign them to the appropriate fields in `chclient.Config`.

**AI Agent Action**: When assisting with Chisel client setup, guide the user to either modify the `default-chisel-client` via the web panel or Telegram bot, or to create new Chisel client configurations as needed. If troubleshooting Chisel client startup, ensure the `Args` field is correctly formatted and that the server address and port are accurate.


## 7. Common Runtime and Configuration Issues

This section covers common errors that may occur after the application is built and running, and how an AI agent can address them.

*   **Error: `Failed to decode remote '--auth': Missing ports` or `Failed to decode remote '--tls': Missing ports`**
    *   **Cause:** The arguments for a Chisel client configuration (e.g., `--auth user:pass` or `--tls`) are being incorrectly passed as part of the remote connection string.
    *   **Solution:** The argument parsing logic in `service/chisel.go` has been fixed to correctly identify `--auth`, `--tls-skip-verify`, and `--tls` as separate flags, filtering them out from the `Remotes` list that is passed to the Chisel library. If this error occurs, ensure the application is running the latest build with this fix.

*   **Error: `http: TLS handshake error: client sent an HTTP request to an HTTPS server` (on server) and `websocket: bad handshake` (on client)**
    *   **Cause:** The Chisel client is attempting to connect using `ws://` (insecure) to a server that expects `wss://` (secure/TLS). This happens when the client configuration in the `s-ui` database is missing a TLS flag.
    *   **Solution:** The client configuration's `Args` field must include `--tls-skip-verify` or `--tls`. The application code has been updated to handle these flags and automatically attempt a `wss://` connection. The default client configuration in `app/app.go` has also been updated to include this flag as an example. Guide the user to update their existing Chisel client configuration to include one of these flags.

*   **Error: `unknown object chisel` when saving from Web UI**
    *   **Cause:** The backend API was missing the logic to handle saving `chisel` objects.
    *   **Solution:** The save logic has been implemented. A `Save` method was added to `service/chisel.go` and a corresponding `case` was added to the main `Save` function in `service/config.go`. If this error occurs, ensure the application is running the latest build.

*   **Error: `unable to load tokens: no such table: tokens` or `no such table: services` on startup**
    *   **Cause:** The user is running a new version of the application with an older database file that is missing the `tokens` and `services` tables.
    *   **Solution:** The database initialization logic in `database/db.go` has been updated to automatically create these tables if they are missing (`AutoMigrate`). This error should be resolved simply by running the latest build of the application.

*   **Error: `UNIQUE constraint failed: chisel_configs.name` when adding a service**
    *   **Cause:** This is a user configuration error. The user is trying to create a Chisel configuration with a name that is already in use.
    *   **Solution:** Instruct the user to either choose a different, unique name for the new service or to first delete the existing service with the same name using the `/remove_chisel <name>` command or the web UI.

*   **Error: `Axios error: request failed with status code: 500` on all pages**
    *   **Cause:** A `nil` pointer panic was occurring in the backend on every API call. This was due to an architectural issue where the `ApiService` was not initialized, causing services like `ChiselService` to have a `nil` database connection.
    *   **Solution:** The `ChiselService` has been refactored to be stateless and use a global database getter, which resolves the panic. This fix is included in the latest build. If this error occurs, ensure the application is running the most recent build.

## 13. GRE Tunneling

The `s-ui` application now supports Generic Routing Encapsulation (GRE) tunneling, allowing for the creation and management of GRE tunnel interfaces directly from the application's API.

### 13.1. Core Architecture

-   **Kernel-Level Integration:** GRE tunnels are created and managed by interacting with the Linux kernel's networking stack via the `vishvananda/netlink` Go library.
-   **Database-Driven Configuration:** GRE tunnel configurations are stored in the `s-ui` SQLite database (`gre_tunnels` table).
-   **Centralized Control:** The `service.GreService` manages the lifecycle of GRE tunnel interfaces.
-   **Privilege Requirement:** Creating and configuring GRE tunnel interfaces requires `CAP_NET_ADMIN` privileges. Therefore, the `s-ui` application must be run with `root` privileges for this functionality to work.

### 13.2. Network Capabilities

-   **Interface Creation:** Dynamically create virtual GRE tunnel interfaces.
-   **IP Configuration:** Assign local and remote IP addresses to the GRE tunnel.
-   **Status Management:** Bring the tunnel interface up or down.

### 13.3. How It Works: From API to Tunnel

1.  **User Action:** A user creates a GRE tunnel configuration via the API.
2.  **API Call:** The frontend sends a request to `POST /api/v2/gre`.
3.  **Service Layer:** The `api.GreAPI` calls `service.GreService.CreateGreTunnel`.
4.  **Database Interaction:** The configuration is saved to the database.
5.  **Netlink Interaction:** `service.GreService` uses `netlink` to:
    a.  Add the GRE tunnel interface (`netlink.LinkAdd`).
    b.  Assign an IP address (`netlink.AddrAdd`).
    c.  Bring the interface up (`netlink.LinkSetUp`).
6.  **Tunnel Established:** The GRE tunnel interface is created and configured in the operating system.

## 14. TAP Tunneling

The `s-ui` application now supports TAP tunneling, allowing for the creation and management of virtual TAP network interfaces.

### 14.1. Core Architecture

-   **User-Space Device Creation:** TAP devices are created using the `songgao/water` Go library, which interacts with `/dev/net/tun` (or equivalent).
-   **Kernel-Level Configuration:** After creation, the TAP device is configured (assigning IP, MTU, bringing up) by interacting with the Linux kernel's networking stack via the `vishvananda/netlink` Go library.
-   **Database-Driven Configuration:** TAP tunnel configurations are stored in the `s-ui` SQLite database (`tap_tunnels` table).
-   **Centralized Control:** The `service.TapService` manages the lifecycle of TAP interfaces.
-   **Privilege Requirement:** While creating the TAP device itself might not always require `root` (depending on `/dev/net/tun` permissions), its full configuration (IP address, MTU, bringing up) requires `CAP_NET_ADMIN` privileges. Therefore, the `s-ui` application must be run with `root` privileges for this functionality to work.

### 14.2. Network Capabilities

-   **Interface Creation:** Dynamically create virtual TAP interfaces.
-   **IP Configuration:** Assign IP addresses to the TAP interface.
-   **MTU Configuration:** Set the Maximum Transmission Unit for the TAP interface.
-   **Status Management:** Bring the TAP interface up or down.
-   **Raw Ethernet Frame Access:** Once created and configured, the TAP interface allows `s-ui` (or another user-space program) to read and write raw Ethernet frames.

### 14.3. How It Works: From API to Interface

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

## 15. MTProto Proxy

The `s-ui` application now supports managing MTProto Proxies for Telegram, allowing users to bypass censorship and access the messaging service. This is implemented by controlling an external `mtg` binary.

### 15.1. Core Architecture

-   **External Process Management:** The `s-ui` application manages the `mtg` (MTProto Proxy) binary as an external process using Go's `os/exec` package.
-   **Database-Driven Configuration:** MTProto Proxy configurations are stored in the `s-ui` SQLite database (`mtproto_proxy_configs` table).
-   **Centralized Control:** The `service.MTProtoService` manages the lifecycle of `mtg` processes (start, stop).
-   **Privilege Requirement:** While `mtg` itself might not require `root` to run (depending on the listen port), `s-ui`'s ability to manage external processes and potentially bind to privileged ports (like 443) might necessitate running `s-ui` with elevated privileges or proper `setcap` configuration for the `sui` binary.

### 15.2. Network Capabilities

-   **MTProto Protocol Support:** Provides a proxy for Telegram's proprietary MTProto protocol.
-   **Censorship Circumvention:** Designed to bypass network restrictions and DPI by mimicking legitimate Telegram traffic.
-   **Obfuscation:** Supports MTProto's built-in obfuscation mechanisms.
-   **Configurable Secret:** Allows setting a unique secret for each proxy instance.
-   **AdTag Support:** Supports optional `AdTag` for promoting Telegram channels.

### 15.3. How It Works: From API to Proxy

1.  **User Action:** A user creates or starts an MTProto Proxy configuration via the API.
2.  **API Call:** The frontend sends a request to `POST /api/v2/mtproto` or `POST /api/v2/mtproto/:id/start`.
3.  **Service Layer:** The `api.MTProtoAPI` calls `service.MTProtoService.StartMTProtoProxy`.
4.  **Database Interaction:** The configuration is saved to the database.
5.  **External Process Launch:** `service.MTProtoService` constructs command-line arguments (e.g., `--bind-to`, `--secret`, `--ad-tag`) and launches the `mtg` binary using `os/exec.CommandContext`.
6.  **Proxy Running:** The `mtg` binary starts listening on the specified port, handling MTProto traffic. `s-ui` monitors its lifecycle and updates its status in the database.

## 16. Privilege Requirements

For the full functionality of GRE, TAP, and MTProto Proxy management, the `s-ui` application requires elevated privileges.

-   **GRE and TAP Tunneling:** Creating and configuring kernel-level network interfaces (GRE) and fully configuring TAP interfaces (assigning IP, MTU, bringing up) requires `CAP_NET_ADMIN` capability.
-   **MTProto Proxy (External Process):** While the `mtg` binary itself might not always need `root` (e.g., if listening on a non-privileged port > 1024), `s-ui`'s ability to manage external processes and potentially bind `mtg` to privileged ports (like 443) might necessitate running `s-ui` with elevated privileges.

**Recommendation:** It is recommended to run the `sui` executable with `root` privileges or configure appropriate Linux capabilities (e.g., `sudo setcap cap_net_admin,cap_net_bind_service=+ep /path/to/sui`) if fine-grained control is desired.