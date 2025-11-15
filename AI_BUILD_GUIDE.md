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
