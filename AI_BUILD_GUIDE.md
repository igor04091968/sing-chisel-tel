# AI Build Guide for GOST UI and Layered Tunnel Setup

This document summarizes the build and deployment steps performed during the session.

## 1. GOST UI Deployment on `vds3.iri1968.dpdns.org`

The `gost-ui` is a Vite-based frontend application. Due to space constraints on `vds3`, it was built locally and then transferred.

### Build Steps (Performed on Agent Host)

1.  **Clone `gost-ui` repository:**
    ```bash
    mkdir -p /tmp/gost-ui-build && cd /tmp/gost-ui-build
    git clone https://github.com/go-gost/gost-ui.git .
    ```
2.  **Install Node.js and npm:**
    ```bash
    sudo apt-get update && sudo apt-get install -y npm
    ```
3.  **Install dependencies and build:**
    ```bash
    cd /tmp/gost-ui-build
    npm install
    npm run build
    ```
    The built files are generated in the `dist` directory.

### Deployment Steps (Performed on `vds3.iri1968.dpdns.org`)

1.  **Package and transfer built files:**
    ```bash
    cd /tmp/gost-ui-build
    tar -czvf gost-ui-dist.tar.gz dist
    # Transfer manually due to 2FA: scp gost-ui-dist.tar.gz root@vds3.iri1968.dpdns.org:/tmp/
    ```
2.  **Deploy files on `vds3`:**
    ```bash
    # On vds3
    rm -rf /var/www/html/gost-ui
    tar -xzvf /tmp/gost-ui-dist.tar.gz -C /var/www/html/
    mv /var/www/html/dist /var/www/html/gost-ui
    chown -R www-data:www-data /var/www/html/gost-ui
    ```
3.  **Configure Nginx:**
    Edit `/etc/nginx/sites-available/sui` and add the following `location` block:
    ```nginx
        location /gost-ui/ {
            root /usr/share/nginx/html; # Corrected path after moving files
            index index.html;
            try_files $uri $uri/ /gost-ui/index.html;
        }
    ```
4.  **Test and reload Nginx:**
    ```bash
    sudo nginx -t
    sudo systemctl reload nginx
    ```
    The UI is accessible at `https://vds3.iri1968.dpdns.org/gost-ui/`.

## 2. Layered Tunnel Setup (GOST + UDP2RAW ICMP)

This setup creates a tunnel where traffic from `vds3` exits to the internet via `gw`, wrapped in `udp2raw` using ICMP mode.

### On `gw.iri1968.dpdns.org` (Server / Exit Node)

1.  **GOST Server (SOCKS5+WSS):**
    ```bash
    /usr/local/bin/gost -L "socks5+wss://127.0.0.1:4443?cert=/root/.acme.sh/gw.iri1968.dpdns.org_ecc/fullchain.cer&key=/root/.acme.sh/gw.iri1968.dpdns.org_ecc/gw.iri1968.dpdns.org.key"
    ```
    *This should be run as a systemd service (e.g., `gost-server.service`).*

2.  **UDP2RAW Server (ICMP):**
    ```bash
    /usr/local/bin/udp2raw -s -l 0.0.0.0:1 -r 127.0.0.1:4443 -k "icmp_password" --raw-mode icmp -a
    ```
    *This should be run as a systemd service.*

### On `vds3.iri1968.dpdns.org` (Client / Traffic Source)

1.  **UDP2RAW Client (ICMP):**
    ```bash
    /usr/local/bin/udp2raw -c -l 127.0.0.1:5555 -r gw.iri1968.dpdns.org -k "icmp_password" --raw-mode icmp -a
    ```
    *This should be run as a systemd service.*

2.  **GOST Client:**
    ```bash
    /usr/local/bin/gost -L socks5://:1081 -F socks5+wss://127.0.0.1:5555 -api :18080
    ```
    *This should be run as a systemd service (e.g., `gost-wss-client.service`).*

### Testing

To test the setup, run on `vds3`:
```bash
curl --socks5 127.0.0.1:1081 ifconfig.me
```
The output should be the IP address of `gw`.
