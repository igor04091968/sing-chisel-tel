# QoS/DSCP Configuration Guide for s-ui

This guide explains how to use the new "DSCP Mark" feature in `s-ui` to apply Quality of Service (QoS) policies to your traffic on Linux systems.

## Concept

The `s-ui` panel now allows you to set a "DSCP Mark" (a number from 0 to 255) on any outbound configuration. This number itself does not change your traffic's priority. Instead, it acts as a label that the Linux kernel can use.

You must configure your server's firewall (`iptables`) to look for this label (or "mark") and then assign a Differentiated Services Code Point (DSCP) value to the packet. The DSCP value is what routers and other network devices use to prioritize traffic.

The flow is as follows:
1.  **`s-ui` -> `sing-box`**: You set a "DSCP Mark" (e.g., `100`) on an outbound in the `s-ui` panel.
2.  **`sing-box`**: When `sing-box` sends a packet through that outbound, it marks the packet with the value `100`.
3.  **Linux Kernel (`iptables`)**: An `iptables` rule sees the packet marked with `100` and sets its DSCP field to a specific value (e.g., `EF` for expedited forwarding).
4.  **Internet**: Routers on the internet see the DSCP value and (if they support QoS) give the packet the corresponding priority.

## How to Configure

### Step 1: Set the Mark in s-ui

1.  Navigate to the **Outbounds** page in your `s-ui` panel.
2.  Create a new outbound or edit an existing one.
3.  In the outbound's configuration, find the **Dial** settings section.
4.  Click the **Options** button to show advanced dialer settings.
5.  Enable the **DSCP Mark** option.
6.  A new "DSCP Mark" field will appear. Enter a number between 1 and 255. This will be your packet mark.
7.  Save the outbound.

### Step 2: Configure iptables on Your Server

You need to SSH into the server where `s-ui` is running and add `iptables` rules. These rules will translate the mark you set in Step 1 into a DSCP value.

The command uses the `mangle` table, which is designed for packet alteration.

**General Command:**
```bash
sudo iptables -t mangle -A OUTPUT -m mark --mark <YOUR_MARK> -j DSCP --set-dscp-class <DSCP_CLASS>
```

-   `<YOUR_MARK>`: The number you entered in the "DSCP Mark" field in `s-ui`.
-   `<DSCP_CLASS>`: A standard DSCP class name.

#### Example: Prioritizing for Low Latency (e.g., Gaming, VoIP)

Let's say you set the **DSCP Mark** to `10` in `s-ui` for a specific outbound. To give this traffic the highest priority and lowest latency, you can use the `EF` (Expedited Forwarding) class.

Run this command on your server:
```bash
sudo iptables -t mangle -A OUTPUT -m mark --mark 10 -j DSCP --set-dscp-class EF
```

#### Example: Prioritizing for High Throughput (e.g., Streaming, Downloads)

Let's say you set the **DSCP Mark** to `20` in `s-ui`. To give this traffic high-throughput priority, you can use one of the `AF` (Assured Forwarding) classes, for example `AF41`.

Run this command on your server:
```bash
sudo iptables -t mangle -A OUTPUT -m mark --mark 20 -j DSCP --set-dscp-class AF41
```

### Step 3: Make Rules Persistent

`iptables` rules are temporary and will be lost on reboot. To make them permanent, you need to use a persistence package.

**For Debian/Ubuntu:**
```bash
# Install the persistence package
sudo apt-get update
sudo apt-get install iptables-persistent

# Save your current rules (both IPv4 and IPv6)
sudo netfilter-persistent save
```

**For CentOS/RHEL/Fedora:**
```bash
# Install the service
sudo yum install iptables-services

# Save the rules
sudo service iptables save
```

### Common DSCP Classes

Here are some common DSCP classes you can use:

| Class Name | Description                   | Use Case                          |
|------------|-------------------------------|-----------------------------------|
| `EF`       | Expedited Forwarding          | Real-time, low-latency (VoIP, gaming) |
| `AF41`     | Assured Forwarding (High Prio) | Video streaming, important data   |
| `AF31`     | Assured Forwarding (Med Prio)  | General important traffic         |
| `AF21`     | Assured Forwarding (Low Prio)  | Less critical traffic             |
| `AF11`     | Assured Forwarding (Lowest Prio)| Background traffic                |
| `CS0`      | Best Effort (Default)         | Normal, non-prioritized traffic   |
| `CS1`      | Scavenger (Lower than Best Effort) | Bulk data, non-critical downloads |

You can also use the numeric value directly with `--set-dscp <value>`. For example, `EF` is `46`.
```bash
sudo iptables -t mangle -A OUTPUT -m mark --mark 10 -j DSCP --set-dscp 46
```
