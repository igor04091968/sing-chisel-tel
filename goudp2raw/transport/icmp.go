package transport

import (
	"errors"
	"net"
	"os"
	"time"

	"goudp2raw/crypto"
	"goudp2raw/raw"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
)

// ICMPConn represents an encrypted tunnel connection over ICMP.
type ICMPConn struct {
	rawConn *raw.Conn
	cipher  *crypto.Cipher
	// ICMP-specific fields
	id  int
	seq int
}

// DialICMP creates a new encrypted ICMP tunnel client connection.
func DialICMP(network, address string, key []byte, dscp int) (*ICMPConn, error) {
	// We expect network to be "ip4:icmp"
	// address can be the destination, e.g., "8.8.8.8"
	// For the raw connection, we listen on all addresses "0.0.0.0"
	rc, err := raw.NewConn(network, "0.0.0.0")
	if err != nil {
		return nil, err
	}

	// Set DSCP if specified
	if dscp > 0 {
		if err := rc.SetDSCP(dscp); err != nil {
			rc.Close()
			return nil, err
		}
	}

	c, err := crypto.NewCipher(key)
	if err != nil {
		rc.Close()
		return nil, err
	}

	return &ICMPConn{
		rawConn: rc,
		cipher:  c,
		id:      os.Getpid() & 0xffff, // Use PID for a pseudo-unique ID
		seq:     0,
	}, nil
}

// ReadFrom reads a packet from the tunnel.
// It reads an ICMP message, decrypts, and returns the payload.
func (c *ICMPConn) ReadFrom(b []byte) (int, net.Addr, error) {
	for {
		// Read from the raw socket
		h, payload, _, err := c.rawConn.ReadFrom()
		if err != nil {
			return 0, nil, err
		}

		// Parse the ICMP message. Protocol 1 is for ICMP on IPv4.
		msg, err := icmp.ParseMessage(1, payload)
		if err != nil {
			continue // Not a valid ICMP message, read again
		}

		// We are only interested in Echo Replies or Requests.
		switch body := msg.Body.(type) {
		case *icmp.Echo:
			// Check if the ID matches our connection
			if body.ID == c.id {
				// Decrypt the data
				plaintext, err := c.cipher.Decrypt(body.Data)
				if err != nil {
					continue // Decryption failed, drop packet
				}
				n := copy(b, plaintext)
				
				// The source address is in the IP header
				fromAddr := &net.IPAddr{IP: h.Src}
				return n, fromAddr, nil
			}
		default:
			continue // Ignore other ICMP types
		}
	}
}

// WriteTo writes a packet to the tunnel.
// It encrypts the data, wraps it in an ICMP Echo message, and sends it.
func (c *ICMPConn) WriteTo(b []byte, addr net.Addr) (int, error) {
	// Encrypt the payload
	ciphertext, err := c.cipher.Encrypt(b)
	if err != nil {
		return 0, err
	}

	c.seq++
	// Create an ICMP Echo message.
	msg := icmp.Message{
		Type: ipv4.ICMPTypeEcho,
		Code: 0,
		Body: &icmp.Echo{
			ID:   c.id,
			Seq:  c.seq,
			Data: ciphertext,
		},
	}

	// Marshal the message into bytes.
	msgBytes, err := msg.Marshal(nil)
	if err != nil {
		return 0, err
	}

	// Write the ICMP packet to the raw connection.
	if err := c.rawConn.WriteTo(msgBytes, addr); err != nil {
		return 0, err
	}

	return len(b), nil
}

// Close closes the underlying raw socket connection.
func (c *ICMPConn) Close() error {
	return c.rawConn.Close()
}

// LocalAddr returns the local network address.
func (c *ICMPConn) LocalAddr() net.Addr {
	return c.rawConn.LocalAddr()
}

// SetDeadline is not implemented.
func (c *ICMPConn) SetDeadline(t time.Time) error {
	return errors.New("not implemented")
}

// SetReadDeadline is not implemented.
func (c *ICMPConn) SetReadDeadline(t time.Time) error {
	return errors.New("not implemented")
}

// SetWriteDeadline is not implemented.
func (c *ICMPConn) SetWriteDeadline(t time.Time) error {
	return errors.New("not implemented")
}
