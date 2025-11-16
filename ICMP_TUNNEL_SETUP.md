# GOST + UDP2RAW ICMP Tunnel Setup

This setup creates a tunnel where traffic from `vds3` exits to the internet via `gw`. The tunnel is wrapped in `udp2raw` using ICMP mode to look like ping traffic.

## On `gw` (Server / Exit Node)

Two processes need to be running.

### 1. GOST Server (WSS + SOCKS5)

Listens locally for a secure websocket connection from `udp2raw`.

`/usr/local/bin/gost -L "socks5+wss://127.0.0.1:4443?cert=/root/.acme.sh/gw.iri1968.dpdns.org_ecc/fullchain.cer&key=/root/.acme.sh/gw.iri1968.dpdns.org_ecc/gw.iri1968.dpdns.org.key"`

### 2. UDP2RAW Server (ICMP)

Listens publicly for ICMP packets and forwards them to the local GOST server.

`/usr/local/bin/udp2raw -s -l 0.0.0.0:1 -r 127.0.0.1:4443 -k "icmp_password" --raw-mode icmp -a`

---

## On `vds3` (Client / Traffic Source)

Two processes need to be running.

### 1. UDP2RAW Client (ICMP)

Connects to the `gw` server via ICMP and creates a local TCP port (`5555`) that mirrors the remote GOST server.

`/usr/local/bin/udp2raw -c -l 127.0.0.1:5555 -r gw.iri1968.dpdns.org -k "icmp_password" --raw-mode icmp -a`

### 2. GOST Client

Creates a local SOCKS5 proxy on port `1081` and forwards its traffic to the local `udp2raw` client.

`/usr/local/bin/gost -L socks5://:1081 -F socks5+wss://127.0.0.1:5555 -api :18080`

---

## Test Command

To test the setup, run on `vds3`:

`curl --socks5 127.0.0.1:1081 ifconfig.me`

The output should be the IP address of `gw`.
