//go:build linux
// +build linux

package raw

import (
	"errors"
	"net"

	"golang.org/x/net/ipv4"
)

// ErrNotImplemented is returned when a feature is not yet implemented.
var ErrNotImplemented = errors.New("not implemented")

// Conn represents a raw socket connection using the high-level ipv4 package.
type Conn struct {
	pconn net.PacketConn
	rconn *ipv4.RawConn
}

// NewConn creates a new raw socket connection.
// Network should be in the form "ip4:icmp" or "ip4:ggp".
func NewConn(network, address string) (*Conn, error) {
	// net.ListenPacket creates a net.PacketConn.
	// For raw sockets, the network should be "ip4:<protocol>".
	p, err := net.ListenPacket(network, address)
	if err != nil {
		return nil, err
	}

	// ipv4.NewRawConn wraps the PacketConn.
	r, err := ipv4.NewRawConn(p)
	if err != nil {
		p.Close()
		return nil, err
	}

	return &Conn{
		pconn: p,
		rconn: r,
	}, nil
}

// ReadFrom reads an IP packet from the connection.
// It returns the IPv4 header, the payload (e.g., the ICMP message),
// and the source address.
func (c *Conn) ReadFrom() (*ipv4.Header, []byte, *ipv4.ControlMessage, error) {
	// Create a buffer large enough for the MTU.
	buf := make([]byte, 1500)

	// Read from the raw connection.
	h, payload, cm, err := c.rconn.ReadFrom(buf)
	if err != nil {
		return nil, nil, nil, err
	}

	// Return the parsed header and the payload.
	return h, payload, cm, nil
}

// WriteTo writes an IP packet to the given address.
// It takes a payload (e.g., an ICMP message) and constructs
// the IP header to send it.
func (c *Conn) WriteTo(payload []byte, to net.Addr) error {
	// Resolve the destination address to an IP.
	ipAddr, err := net.ResolveIPAddr("ip4", to.String())
	if err != nil {
		return err
	}

	// Create a basic IPv4 header.
	// The kernel will fill in the TotalLen, ID, and Checksum.
	h := &ipv4.Header{
		Version:  ipv4.Version,
		Len:      ipv4.HeaderLen,
		Protocol: 1, // 1 for ICMP
		Flags:    ipv4.DontFragment,
		TTL:      64,
		Dst:      ipAddr.IP,
		// Src can be left as nil, the kernel will fill it in.
	}

	// Write the packet.
	if err := c.rconn.WriteTo(h, payload, nil); err != nil {
		return err
	}

	return nil
}

// Close closes the connection.
func (c *Conn) Close() error {
	return c.rconn.Close()
}

// LocalAddr returns the local network address.
func (c *Conn) LocalAddr() net.Addr {
	return c.pconn.LocalAddr()
}

// SetDSCP sets the Differentiated Services Code Point (DSCP) value for QoS.
// The value is set in the Type of Service (TOS) field of the IP header.
func (c *Conn) SetDSCP(dscp int) error {
	// The TOS field is 8 bits. DSCP is the high 6 bits.
	// We shift the dscp value left by 2 to place it in the correct position.
	// This is how the underlying ipv4.RawConn.SetTOS expects the value.
	return c.rconn.SetTOS(dscp << 2)
}
