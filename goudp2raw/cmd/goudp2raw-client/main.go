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
	localAddr := flag.String("l", "127.0.0.1:8888", "Local address to listen for UDP packets")
	remoteAddr := flag.String("r", "server.example.com", "Address of the goudp2raw server")
	key := flag.String("k", "defaultkey", "Pre-shared key for encryption")
	dscp := flag.Int("dscp", 0, "DSCP value for QoS (e.g., 46 for EF)")
	flag.Parse()

	log.Println("Starting goudp2raw client...")
	log.Printf("Listening for UDP on: %s", *localAddr)
	log.Printf("Tunnelling to server: %s", *remoteAddr)
	if *dscp > 0 {
		log.Printf("Setting DSCP to: %d", *dscp)
	}

	// Resolve the server's address
	serverIP, err := net.ResolveIPAddr("ip4", *remoteAddr)
	if err != nil {
		log.Fatalf("Failed to resolve server address: %v", err)
	}

	// Create the ICMP tunnel connection.
	icmpConn, err := transport.DialICMP("ip4:icmp", "0.0.0.0", []byte(*key), *dscp)
	if err != nil {
		log.Fatalf("Failed to create ICMP tunnel: %v", err)
	}
	defer icmpConn.Close()

	// Create a standard UDP connection to listen for the local application
	localUDPAddr, err := net.ResolveUDPAddr("udp", *localAddr)
	if err != nil {
		log.Fatalf("Failed to resolve local UDP address: %v", err)
	}
	udpConn, err := net.ListenUDP("udp", localUDPAddr)
	if err != nil {
		log.Fatalf("Failed to listen on local UDP address: %v", err)
	}
	defer udpConn.Close()

	log.Println("Client started successfully. Waiting for traffic...")

	// Bidirectional traffic forwarding
	var lastAppAddr net.Addr // Remember where the local app sent from

	// 1. From local app to ICMP tunnel (server)
	go func() {
		buf := make([]byte, 4096)
		for {
			n, from, err := udpConn.ReadFrom(buf)
			if err != nil {
				log.Printf("Error reading from local UDP: %v", err)
				continue
			}
			lastAppAddr = from // Remember the source
			log.Printf("Received %d bytes from local app", n)

			// Forward the encrypted packet to the server
			if _, err := icmpConn.WriteTo(buf[:n], serverIP); err != nil {
				log.Printf("Error forwarding packet to ICMP server: %v", err)
			}
		}
	}()

	// 2. From ICMP tunnel (server) back to local app
	go func() {
		buf := make([]byte, 4096)
		for {
			n, _, err := icmpConn.ReadFrom(buf)
			if err != nil {
				log.Printf("Error reading from ICMP tunnel: %v", err)
				continue
			}
			log.Printf("Received %d bytes from server", n)

			if lastAppAddr != nil {
				// Forward the decrypted packet back to the local app
				if _, err := udpConn.WriteTo(buf[:n], lastAppAddr); err != nil {
					log.Printf("Error forwarding packet to local app: %v", err)
				}
			}
		}
	}()

	// Wait for a signal to exit
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
	log.Println("Shutting down client...")
}
