package goudp2raw

import "net"

// Tunnel defines the basic interface for a udp2raw tunnel.
// It behaves like a net.PacketConn.
type Tunnel interface {
	net.PacketConn
	// Close closes the tunnel.
	Close() error
}

// Server defines the interface for a tunnel server.
type Server interface {
	// ListenAndServe starts listening and serving the tunnel.
	ListenAndServe() error
	// Close stops the server.
	Close() error
}

// Client defines the interface for a tunnel client.
type Client interface {
	// Dial connects to the remote server and returns a tunnel connection.
	Dial() (Tunnel, error)
}

// Config holds the common configuration for a tunnel.
type Config struct {
	// LocalAddr is the local address to bind to.
	// For a client, this is where it listens for incoming UDP packets.
	// For a server, this is the public-facing address for the raw socket.
	LocalAddr string

	// RemoteAddr is the remote address.
	// For a client, this is the address of the tunnel server.
	// For a server, this is the address where UDP packets are forwarded to.
	RemoteAddr string

	// Key is the pre-shared key for encryption.
	Key string

	// RawMode specifies the transport protocol for the raw socket.
	// e.g., "icmp", "faketcp", "udp".
	RawMode string

	// DSCP is the Differentiated Services Code Point value to set on outgoing packets.
	// This is used for QoS.
	DSCP int

	// Additional protocol-specific arguments.
	Args map[string]string
}
