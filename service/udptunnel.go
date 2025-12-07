package service

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"net"
	"sync"
	"syscall"

	"github.com/igor04091968/sing-chisel-tel/database"
	"github.com/igor04091968/sing-chisel-tel/database/model"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"gorm.io/gorm"
)

// UdpTunnelService manages UDP tunnels with udp2raw features.
type UdpTunnelService struct {
	db             *gorm.DB
	runningTunnels map[uint]*UdpTunnelInstance
	mu             sync.Mutex
}

// UdpTunnelInstance holds the context and cancel function for a running tunnel
type UdpTunnelInstance struct {
	Cancel context.CancelFunc
	Config *model.UdpTunnelConfig
}

// NewUdpTunnelService creates a new UdpTunnelService
func NewUdpTunnelService(db *gorm.DB) *UdpTunnelService {
	return &UdpTunnelService{
		db:             db,
		runningTunnels: make(map[uint]*UdpTunnelInstance),
	}
}

// StartUdpTunnel starts a UDP tunnel based on the provided configuration.
func (s *UdpTunnelService) StartUdpTunnel(cfg *model.UdpTunnelConfig) error {
	s.mu.Lock()
	if _, ok := s.runningTunnels[cfg.ID]; ok {
		s.mu.Unlock()
		return fmt.Errorf("UDP tunnel %s is already running", cfg.Name)
	}

	ctx, cancel := context.WithCancel(context.Background())
	instance := &UdpTunnelInstance{
		Cancel: cancel,
		Config: cfg,
	}
	s.runningTunnels[cfg.ID] = instance
	s.mu.Unlock()

	log.Printf("Starting pure Go UDP tunnel %s (Mode: %s, Role: %s)", cfg.Name, cfg.Mode, cfg.Role)

	go func() {
		err := s.runTunnel(ctx, cfg) // Corrected function name
		if err != nil {
			log.Printf("Error running tunnel %s: %v", cfg.Name, err)
		}

		// Cleanup
		s.mu.Lock()
		defer s.mu.Unlock()

		delete(s.runningTunnels, cfg.ID)

		var dbCfg model.UdpTunnelConfig
		database := database.GetDB()
		if err := database.First(&dbCfg, cfg.ID).Error; err == nil {
			if dbCfg.Status == "running" {
				dbCfg.Status = "stopped"
				dbCfg.ProcessID = 0
				database.Save(&dbCfg)
			}
		}
	}()

	cfg.Status = "running"
	cfg.ProcessID = 1
	return s.db.Save(cfg).Error
}

// runTunnelClient contains the core logic for the pure Go udp2raw implementation for client mode.
func (s *UdpTunnelService) runTunnelClient(ctx context.Context, cfg *model.UdpTunnelConfig) error {
	// 1. Listen for incoming UDP packets from the local application
	localAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("127.0.0.1:%d", cfg.ListenPort))
	if err != nil {
		return fmt.Errorf("failed to resolve local UDP address: %w", err)
	}
	conn, err := net.ListenUDP("udp", localAddr)
	if err != nil {
		return fmt.Errorf("failed to listen on local UDP port: %w", err)
	}
	defer conn.Close()

	// 2. Set up the raw socket for sending crafted packets
	destIP, destPort, err := parseRemoteAddress(cfg.RemoteAddress)
	if err != nil {
		return err
	}

	fd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_RAW, syscall.IPPROTO_RAW)
	if err != nil {
		return fmt.Errorf("failed to create raw socket (requires root): %w", err)
	}
	defer syscall.Close(fd)

	// 3. Set DSCP value on the socket
	tos := int(cfg.DSCP) << 2
	if err := syscall.SetsockoptInt(fd, syscall.IPPROTO_IP, syscall.IP_TOS, tos); err != nil {
		return fmt.Errorf("failed to set DSCP (IP_TOS) on raw socket: %w", err)
	}

	// Tell the kernel that we will provide our own IP header
	if err := syscall.SetsockoptInt(fd, syscall.IPPROTO_IP, syscall.IP_HDRINCL, 1); err != nil {
		return fmt.Errorf("failed to set IP_HDRINCL on raw socket: %w", err)
	}

	// Main loop: read from UDP, craft packet, send on raw socket
	buffer := make([]byte, 1500)
	for {
		select {
		case <-ctx.Done():
			return nil // Tunnel stopped
		default:
			n, _, err := conn.ReadFromUDP(buffer)
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue
				}
				return err
			}
			payload := buffer[:n]

			switch cfg.Mode {
			case "faketcp":
				err = sendFakeTCPPacket(fd, destIP, destPort, payload, tos)
			case "icmp":
				err = sendICMPPacket(fd, destIP, payload, tos)
			case "raw_udp":
				err = sendRawUDPPacket(fd, destIP, destPort, payload, tos)
			default:
				err = fmt.Errorf("unsupported tunnel mode: %s", cfg.Mode)
			}

			if err != nil {
				log.Printf("Failed to send packet in mode %s: %v", cfg.Mode, err)
			}
		}
	}
}



// runTunnel is a dispatcher for client and server tunnel modes.
func (s *UdpTunnelService) runTunnel(ctx context.Context, cfg *model.UdpTunnelConfig) error {
	switch cfg.Role {
	case "client":
		return s.runTunnelClient(ctx, cfg)
	case "server":
		return s.runTunnelServer(ctx, cfg)
	default:
		return fmt.Errorf("unsupported UDP tunnel role: %s", cfg.Role)
	}
}

// runTunnelServer contains the core logic for the pure Go udp2raw implementation for server mode.
func (s *UdpTunnelService) runTunnelServer(ctx context.Context, cfg *model.UdpTunnelConfig) error {
	log.Printf("Starting UDP tunnel server %s (Mode: %s, Listen Port: %d, Remote Address: %s)", cfg.Name, cfg.ListenPort, cfg.RemoteAddress)

	var proto int
	switch cfg.Mode {
	case "faketcp":
		proto = syscall.IPPROTO_TCP
	case "icmp":
		proto = syscall.IPPROTO_ICMP
	case "raw_udp":
		proto = syscall.IPPROTO_UDP
	default:
		return fmt.Errorf("unsupported server tunnel mode: %s for config %s", cfg.Mode, cfg.Name)
	}

	// Create a raw socket to listen for incoming IP packets
	// This requires CAP_NET_RAW capability.
	fd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_RAW, proto)
	if err != nil {
		return fmt.Errorf("failed to create raw socket for server (requires CAP_NET_RAW): %w", err)
	}
	defer syscall.Close(fd)

	// Use a buffer to read raw IP packets
	packetBuffer := make([]byte, 65536) // Max IP packet size

	// Create a UDP connection to the remote address for forwarding decoded packets
	remoteUDPAddr, err := net.ResolveUDPAddr("udp", cfg.RemoteAddress)
	if err != nil {
		return fmt.Errorf("failed to resolve remote UDP address for forwarding: %w", err)
	}
	remoteUDPConn, err := net.DialUDP("udp", nil, remoteUDPAddr)
	if err != nil {
		return fmt.Errorf("failed to dial remote UDP address for forwarding: %w", err)
	}
	defer remoteUDPConn.Close()

	for {
		select {
		case <-ctx.Done():
			log.Printf("UDP tunnel server %s stopped.", cfg.Name)
			return nil
		default:
			// Read raw IP packets
			n, _, err := syscall.Recvfrom(fd, packetBuffer, 0)
			if err != nil {
				log.Printf("Error reading from raw socket for server %s: %v", cfg.Name, err)
				continue
			}

			// Add a log to confirm raw packet reception
			log.Printf("SERVER %s: Received raw packet of size %d", cfg.Name, n)

			// Process the raw IP packet
			packet := gopacket.NewPacket(packetBuffer[:n], layers.LayerTypeIPv4, gopacket.Default)
			ipLayer := packet.Layer(layers.LayerTypeIPv4)

			if ipLayer == nil {
				log.Printf("SERVER %s: Could not decode IPv4 layer", cfg.Name)
				continue
			}

			ip := ipLayer.(*layers.IPv4)
			var udpPayload []byte

			switch cfg.Mode {
			case "faketcp":
				tcpLayer := packet.Layer(layers.LayerTypeTCP)
				if tcpLayer != nil {
					tcp := tcpLayer.(*layers.TCP)

					log.Printf("SERVER %s: Parsed TCP packet. DstIP: %s, DstPort: %d, Flags: SYN=%t, ACK=%t, PSH=%t, URG=%t, FIN=%t, RST=%t, SrcIP: %s, SrcPort: %d",
						cfg.Name, ip.DstIP, tcp.DstPort, tcp.SYN, tcp.ACK, tcp.PSH, tcp.URG, tcp.FIN, tcp.RST, ip.SrcIP, tcp.SrcPort)

					// Filter packets by destination port (cfg.ListenPort)
					// and check if it resembles a faketcp packet.
					// For a simple faketcp server, we assume payload is UDP.
					if uint16(tcp.DstPort) == uint16(cfg.ListenPort) {
						if tcp.SYN && !tcp.ACK { // Client initiating handshake
							log.Printf("Received SYN packet on Listen Port %d from %s:%d. Sending SYN-ACK...",
								cfg.ListenPort, ip.SrcIP, tcp.SrcPort)

							// Craft and send SYN-ACK
							err := s.sendFakeTCPSynAck(fd, ip, tcp)
							if err != nil {
								log.Printf("Error sending SYN-ACK for server %s: %v", cfg.Name, err)
							}
						} else if tcp.ACK && len(tcp.Payload) > 0 { // Client has completed handshake and sending data
							// Assume the payload is the encapsulated UDP data
							udpPayload = tcp.Payload

							// Forward the UDP payload to the remote UDP address
							_, err := remoteUDPConn.Write(udpPayload)
							if err != nil {
								log.Printf("Error forwarding UDP payload for server %s: %v", cfg.Name, err)
							} else {
								log.Printf("Forwarded %d bytes of UDP payload for server %s to %s",
									len(udpPayload), cfg.Name, cfg.RemoteAddress)
								// TODO: Respond to the client with an ACK for the received data (optional for faketcp)
							}
						} else {
							log.Printf("Received unhandled TCP packet on Listen Port %d (Flags: SYN=%t, ACK=%t, PayloadLen=%d)",
								cfg.ListenPort, tcp.SYN, tcp.ACK, len(tcp.Payload))
						}
					}
				}
			case "icmp":
				icmpLayer := packet.Layer(layers.LayerTypeICMPv4)
				if icmpLayer != nil {
					icmp := icmpLayer.(*layers.ICMPv4)
					log.Printf("SERVER %s: Parsed ICMP packet. TypeCode: %s, ID: %d, Seq: %d, SrcIP: %s",
						cfg.Name, icmp.TypeCode, icmp.Id, icmp.Seq, ip.SrcIP)

					// We expect ICMP Echo Request with UDP payload
					if icmp.TypeCode.Type() == layers.ICMPv4TypeEchoRequest && len(icmp.Payload) > 0 {
						udpPayload = icmp.Payload // Assume payload is encapsulated UDP

						log.Printf("SERVER %s: Received ICMP Echo Request with %d bytes of payload from %s. Sending Echo Reply...",
							cfg.Name, len(udpPayload), ip.SrcIP)

						err := s.sendICMPEchoReply(fd, ip, icmp)
						if err != nil {
							log.Printf("Error sending ICMP Echo Reply for server %s: %v", cfg.Name, err)
						}

						// Forward the UDP payload
						_, err = remoteUDPConn.Write(udpPayload)
						if err != nil {
							log.Printf("Error forwarding UDP payload (ICMP mode) for server %s: %v", cfg.Name, err)
						} else {
							log.Printf("Forwarded %d bytes of UDP payload (ICMP mode) for server %s to %s",
								len(udpPayload), cfg.Name, cfg.RemoteAddress)
						}
					} else {
						log.Printf("SERVER %s: Received unhandled ICMP packet (TypeCode: %s, PayloadLen: %d)",
							cfg.Name, icmp.TypeCode, len(icmp.Payload))
					}
				}
			case "raw_udp":
				udpLayer := packet.Layer(layers.LayerTypeUDP)
				if udpLayer != nil {
					udp := udpLayer.(*layers.UDP)
					log.Printf("SERVER %s: Parsed UDP packet. SrcPort: %d, DstPort: %d, Len: %d, SrcIP: %s",
						cfg.Name, udp.SrcPort, udp.DstPort, udp.Length, ip.SrcIP)

					// Filter by ListenPort if needed, otherwise just forward
					if uint16(udp.DstPort) == uint16(cfg.ListenPort) {
						udpPayload = udp.Payload

						log.Printf("SERVER %s: Received raw UDP packet with %d bytes of payload from %s:%d. Forwarding...",
							cfg.Name, len(udpPayload), ip.SrcIP, udp.SrcPort)

						// Forward the UDP payload
						_, err := remoteUDPConn.Write(udpPayload)
						if err != nil {
							log.Printf("Error forwarding UDP payload (raw_udp mode) for server %s: %v", cfg.Name, err)
						} else {
							log.Printf("Forwarded %d bytes of UDP payload (raw_udp mode) for server %s to %s",
								len(udpPayload), cfg.Name, cfg.RemoteAddress)
						}
					} else {
						log.Printf("SERVER %s: Received raw UDP packet on unexpected port %d (expected %d)",
							cfg.Name, udp.DstPort, cfg.ListenPort)
					}
				}
			}
		}
	}
}

// sendFakeTCPPacket crafts and sends a TCP packet with the given payload.
func sendFakeTCPPacket(fd int, destIP net.IP, destPort uint16, payload []byte, tos int) error {
	// This is a simplified example. A real implementation needs to get the source IP properly.
	srcIP := net.ParseIP("127.0.0.1") // Placeholder, should be the outbound IP

	// Craft the packet layers
	ipLayer := &layers.IPv4{
		Version:  4,
		IHL:      5,
		TOS:      uint8(tos),
		Length:   20 + 20 + uint16(len(payload)), // IP header + TCP header + payload
		Id:       uint16(rand.Intn(65535)),
		Flags:    layers.IPv4DontFragment,
		TTL:      64,
		Protocol: layers.IPProtocolTCP,
		SrcIP:    srcIP,
		DstIP:    destIP,
	}
	tcpLayer := &layers.TCP{
		SrcPort: layers.TCPPort(rand.Intn(65535-1024) + 1024),
		DstPort: layers.TCPPort(destPort),
		SYN:     true, // This makes it "FakeTCP"
		Window:  14600,
		Seq:     rand.Uint32(),
	}
	tcpLayer.SetNetworkLayerForChecksum(ipLayer)

	// Serialize the packet
	buf := gopacket.NewSerializeBuffer()
	opts := gopacket.SerializeOptions{
		ComputeChecksums: true,
		FixLengths:       true,
	}
	if err := gopacket.SerializeLayers(buf, opts, ipLayer, tcpLayer, gopacket.Payload(payload)); err != nil {
		return fmt.Errorf("failed to serialize packet: %w", err)
	}

	// Send the packet
	addr := syscall.SockaddrInet4{
		Port: 0, // Port is in the TCP header
	}
	copy(addr.Addr[:], destIP.To4())

	return syscall.Sendto(fd, buf.Bytes(), 0, &addr)
}

// sendFakeTCPSynAck crafts and sends a SYN-ACK TCP packet in response to a received SYN.
func (s *UdpTunnelService) sendFakeTCPSynAck(fd int, clientIP *layers.IPv4, clientTCP *layers.TCP) error {
	// Source IP of the response should be the destination IP of the incoming packet
	srcIP := clientIP.DstIP
	// Destination IP of the response should be the source IP of the incoming packet
	dstIP := clientIP.SrcIP

	// Craft the packet layers for SYN-ACK
	ipLayer := &layers.IPv4{
		Version:  4,
		IHL:      5,
		TOS:      clientIP.TOS, // Maintain same TOS
		Length:   20 + 20,      // IP header + TCP header (no payload for SYN-ACK)
		Id:       uint16(rand.Intn(65535)),
		Flags:    layers.IPv4DontFragment,
		TTL:      64,
		Protocol: layers.IPProtocolTCP,
		SrcIP:    srcIP,
		DstIP:    dstIP,
	}
	tcpLayer := &layers.TCP{
		SrcPort: clientTCP.DstPort, // Server's listen port
		DstPort: clientTCP.SrcPort, // Client's source port
		ACK:     true,
		SYN:     true,
		Window:  14600,
		Seq:     rand.Uint32(),                      // Server's initial sequence number
		Ack:     clientTCP.Seq + 1,                  // Acknowledge client's sequence number
	}
	tcpLayer.SetNetworkLayerForChecksum(ipLayer)

	// Serialize the packet
	buf := gopacket.NewSerializeBuffer()
	opts := gopacket.SerializeOptions{
		ComputeChecksums: true,
		FixLengths:       true,
	}
	if err := gopacket.SerializeLayers(buf, opts, ipLayer, tcpLayer); err != nil {
		return fmt.Errorf("failed to serialize SYN-ACK packet: %w", err)
	}

	// Send the packet using the raw socket
	addr := syscall.SockaddrInet4{
		Port: 0, // Port is in the TCP header
	}
	copy(addr.Addr[:], dstIP.To4())

	return syscall.Sendto(fd, buf.Bytes(), 0, &addr)
}

// sendICMPEchoReply crafts and sends an ICMP Echo Reply packet.
func (s *UdpTunnelService) sendICMPEchoReply(fd int, clientIP *layers.IPv4, clientICMP *layers.ICMPv4) error {
	// Source IP of the response should be the destination IP of the incoming packet
	srcIP := clientIP.DstIP
	// Destination IP of the response should be the source IP of the incoming packet
	dstIP := clientIP.SrcIP

	// Craft the packet layers for ICMP Echo Reply
	ipLayer := &layers.IPv4{
		Version:  4,
		IHL:      5,
		TOS:      clientIP.TOS, // Maintain same TOS
		Length:   20 + 8 + uint16(len(clientICMP.Payload)), // IP header + ICMP header + payload
		Id:       uint16(rand.Intn(65535)),
		Flags:    layers.IPv4DontFragment,
		TTL:      64,
		Protocol: layers.IPProtocolICMPv4,
		SrcIP:    srcIP,
		DstIP:    dstIP,
	}
	icmpLayer := &layers.ICMPv4{
		TypeCode: layers.CreateICMPv4TypeCode(layers.ICMPv4TypeEchoReply, 0),
		Id:       clientICMP.Id,
		Seq:      clientICMP.Seq,
		// In gopacket v1.1.19, Payload is not a direct field of ICMPv4.
		// It's part of the BaseLayer and passed separately for serialization.
	}
	// Per gopacket v1.1.19, the checksum is calculated by SerializeLayers when ComputeChecksums is true.
	// icmpLayer.SetNetworkLayerForChecksum is not available in this version.

	// Serialize the packet
	buf := gopacket.NewSerializeBuffer()
	opts := gopacket.SerializeOptions{
		ComputeChecksums: true,
		FixLengths:       true,
	}
	if err := gopacket.SerializeLayers(buf, opts, ipLayer, icmpLayer, gopacket.Payload(clientICMP.Payload)); err != nil {
		return fmt.Errorf("failed to serialize ICMP Echo Reply packet: %w", err)
	}

	// Send the packet using the raw socket
	addr := syscall.SockaddrInet4{
		Port: 0, // Port is in the ICMP header
	}
	copy(addr.Addr[:], dstIP.To4())

	return syscall.Sendto(fd, buf.Bytes(), 0, &addr)
}

// sendICMPPacket crafts and sends an ICMP Echo Request packet with the given payload.
func sendICMPPacket(fd int, destIP net.IP, payload []byte, tos int) error {
	// This is a simplified example. A real implementation needs to get the source IP properly.
	srcIP := net.ParseIP("127.0.0.1") // Placeholder, should be the outbound IP

	// Craft the packet layers
	ipLayer := &layers.IPv4{
		Version:  4,
		IHL:      5,
		TOS:      uint8(tos),
		Length:   20 + 8 + uint16(len(payload)), // IP header + ICMP header + payload
		Id:       uint16(rand.Intn(65535)),
		Flags:    layers.IPv4DontFragment,
		TTL:      64,
		Protocol: layers.IPProtocolICMPv4,
		SrcIP:    srcIP,
		DstIP:    destIP,
	}
	icmpLayer := &layers.ICMPv4{
		TypeCode: layers.CreateICMPv4TypeCode(layers.ICMPv4TypeEchoRequest, 0),
		Id:       uint16(rand.Intn(65535)),
		Seq:      uint16(rand.Intn(65535)),
		// In gopacket v1.1.19, Payload is not a direct field of ICMPv4.
		// It's part of the BaseLayer and passed separately for serialization.
	}
	// Per gopacket v1.1.19, the checksum is calculated by SerializeLayers when ComputeChecksums is true.
	// icmpLayer.SetNetworkLayerForChecksum is not available in this version.

	// Serialize the packet
	buf := gopacket.NewSerializeBuffer()
	opts := gopacket.SerializeOptions{
		ComputeChecksums: true,
		FixLengths:       true,
	}
	if err := gopacket.SerializeLayers(buf, opts, ipLayer, icmpLayer, gopacket.Payload(payload)); err != nil {
		return fmt.Errorf("failed to serialize ICMP packet: %w", err)
	}

	// Send the packet
	addr := syscall.SockaddrInet4{
		Port: 0, // Port is in the ICMP header
	}
	copy(addr.Addr[:], destIP.To4())

	return syscall.Sendto(fd, buf.Bytes(), 0, &addr)
}

// sendRawUDPPacket crafts and sends a raw UDP packet with the given payload.
func sendRawUDPPacket(fd int, destIP net.IP, destPort uint16, payload []byte, tos int) error {
	// This is a simplified example. A real implementation needs to get the source IP properly.
	srcIP := net.ParseIP("127.0.0.1") // Placeholder, should be the outbound IP
	srcPort := uint16(rand.Intn(65535-1024) + 1024) // Random source port

	// Craft the packet layers
	ipLayer := &layers.IPv4{
		Version:  4,
		IHL:      5,
		TOS:      uint8(tos),
		Length:   20 + 8 + uint16(len(payload)), // IP header + UDP header + payload
		Id:       uint16(rand.Intn(65535)),
		Flags:    layers.IPv4DontFragment,
		TTL:      64,
		Protocol: layers.IPProtocolUDP,
		SrcIP:    srcIP,
		DstIP:    destIP,
	}
	udpLayer := &layers.UDP{
		SrcPort: layers.UDPPort(srcPort),
		DstPort: layers.UDPPort(destPort),
		Length:  uint16(8 + len(payload)), // UDP header length + payload length
	}
	udpLayer.SetNetworkLayerForChecksum(ipLayer)

	// Serialize the packet
	buf := gopacket.NewSerializeBuffer()
	opts := gopacket.SerializeOptions{
		ComputeChecksums: true,
		FixLengths:       true,
	}
	if err := gopacket.SerializeLayers(buf, opts, ipLayer, udpLayer, gopacket.Payload(payload)); err != nil {
		return fmt.Errorf("failed to serialize raw UDP packet: %w", err)
	}

	// Send the packet
	addr := syscall.SockaddrInet4{
		Port: 0, // Port is in the UDP header
	}
	copy(addr.Addr[:], destIP.To4())

	return syscall.Sendto(fd, buf.Bytes(), 0, &addr)
}

func parseRemoteAddress(addr string) (net.IP, uint16, error) {
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, 0, fmt.Errorf("invalid remote address format: %w", err)
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return nil, 0, fmt.Errorf("invalid remote IP address: %s", host)
	}
	port, err := net.LookupPort("tcp", portStr)
	if err != nil {
		return nil, 0, fmt.Errorf("invalid remote port: %w", err)
	}
	return ip, uint16(port), nil
}


// StopUdpTunnel stops a running UDP tunnel.
func (s *UdpTunnelService) StopUdpTunnel(id uint) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	instance, ok := s.runningTunnels[id]
	if !ok {
		return fmt.Errorf("UDP tunnel with ID %d is not running", id)
	}

	instance.Cancel() // Signal the goroutine to stop
	delete(s.runningTunnels, id)

	var cfg model.UdpTunnelConfig
	if err := s.db.First(&cfg, id).Error; err == nil {
		cfg.Status = "stopped"
		cfg.ProcessID = 0
		return s.db.Save(&cfg).Error
	}

	return nil
}

// ... (rest of the CRUD functions remain the same)
func (s *UdpTunnelService) GetAllUdpTunnels() ([]model.UdpTunnelConfig, error) {
	var tunnels []model.UdpTunnelConfig
	err := s.db.Find(&tunnels).Error
	return tunnels, err
}

func (s *UdpTunnelService) GetUdpTunnelByID(id uint) (*model.UdpTunnelConfig, error) {
	var cfg model.UdpTunnelConfig
	err := s.db.First(&cfg, id).Error
	return &cfg, err
}

func (s *UdpTunnelService) GetUdpTunnelByName(name string) (*model.UdpTunnelConfig, error) {
	var config model.UdpTunnelConfig
	err := s.db.Where("name = ?", name).First(&config).Error
	return &config, err
}

func (s *UdpTunnelService) CreateUdpTunnel(cfg *model.UdpTunnelConfig) error {
	return s.db.Create(cfg).Error
}

func (s *UdpTunnelService) UpdateUdpTunnel(cfg *model.UdpTunnelConfig) error {
	return s.db.Save(cfg).Error
}

func (s *UdpTunnelService) DeleteUdpTunnel(id uint) error {
	if err := s.StopUdpTunnel(id); err != nil {
		log.Printf("Tunnel %d was not running, deleting from DB.", id)
	}
	return s.db.Delete(&model.UdpTunnelConfig{}, id).Error
}

func (s *UdpTunnelService) AutoStartUdpTunnels() {
	var tunnels []model.UdpTunnelConfig
	if err := s.db.Where("status = ?", "running").Find(&tunnels).Error; err != nil {
		fmt.Printf("Error retrieving UDP tunnels for autostart: %v\n", err)
		return
	}

	for i := range tunnels {
		log.Printf("Autostarting UDP tunnel: %s", tunnels[i].Name)
		tunnelToStart := tunnels[i]
		if err := s.StartUdpTunnel(&tunnelToStart); err != nil {
			log.Printf("Error autostarting UDP tunnel %s: %v", tunnelToStart.Name, err)
		}
	}
}

