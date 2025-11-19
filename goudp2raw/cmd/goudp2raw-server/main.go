package main

import (
	"flag"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"goudp2raw/transport"
)

func main() {
	// Command-line flags
	localAddr := flag.String("l", "0.0.0.0", "Public-facing address for the raw socket listener")
	remoteAddr := flag.String("r", "127.0.0.1:7777", "Address of the target UDP service (e.g., game server)")
	key := flag.String("k", "defaultkey", "Pre-shared key for encryption")
	dscp := flag.Int("dscp", 0, "DSCP value for QoS (e.g., 46 for EF)")
	flag.Parse()

	log.Println("Starting goudp2raw server...")
	log.Printf("Raw ICMP listener on: %s", *localAddr)
	log.Printf("Forwarding UDP to: %s", *remoteAddr)
	if *dscp > 0 {
		log.Printf("Setting DSCP to: %d", *dscp)
	}

	// Resolve the target UDP service address
	udpAddr, err := net.ResolveUDPAddr("udp", *remoteAddr)
	if err != nil {
		log.Fatalf("Failed to resolve remote UDP address: %v", err)
	}

	// Create the ICMP tunnel connection.
	// The address here is not used for dialing, but for listening.
	// The underlying raw socket will bind to the address specified in NewConn.
	icmpConn, err := transport.DialICMP("ip4:icmp", *localAddr, []byte(*key), *dscp)
	if err != nil {
		log.Fatalf("Failed to create ICMP tunnel: %v", err)
	}
	defer icmpConn.Close()

	// Create a standard UDP connection to the target service
	udpConn, err := net.ListenPacket("udp", "0.0.0.0:0") // Listen on a random port
	if err != nil {
		log.Fatalf("Failed to create UDP connection: %v", err)
	}
	defer udpConn.Close()

	log.Println("Server started successfully. Waiting for traffic...")

	// Bidirectional traffic forwarding
	clientAddrs := make(map[string]net.Addr) // Maps client's real IP to its tunnel addr

	// 1. From ICMP tunnel (client) to UDP service (game server)
	go func() {
		buf := make([]byte, 4096)
		for {
			n, from, err := icmpConn.ReadFrom(buf)
			if err != nil {
				log.Printf("Error reading from ICMP tunnel: %v", err)
				continue
			}
			clientAddrs[from.String()] = from // Store client address
			log.Printf("Received %d bytes from client %s", n, from.String())

			// Forward the decrypted packet to the UDP service
			if _, err := udpConn.WriteTo(buf[:n], udpAddr); err != nil {
				log.Printf("Error forwarding packet to UDP service: %v", err)
			}
		}
	}()

	// 2. From UDP service (game server) back to ICMP tunnel (client)
	go func() {
		buf := make([]byte, 4096)
		for {
			n, _, err := udpConn.ReadFrom(buf) // We don't need the addr, it's the game server
			if err != nil {
				log.Printf("Error reading from UDP service: %v", err)
				continue
			}
			log.Printf("Received %d bytes from UDP service", n)

			// This is a simple implementation: forward to the last seen client.
			// A real implementation would need more sophisticated session management.
			var lastClientAddr net.Addr
			for _, addr := range clientAddrs {
				lastClientAddr = addr
			}

			if lastClientAddr != nil {
				// Forward the encrypted packet back to the client
				if _, err := icmpConn.WriteTo(buf[:n], lastClientAddr); err != nil {
					log.Printf("Error forwarding packet to ICMP client: %v", err)
				}
			}
		}
	}()

	// Wait for a signal to exit
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
	log.Println("Shutting down server...")
}
