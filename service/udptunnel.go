package service

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"
	"syscall"

	"github.com/alireza0/s-ui/database"
	"github.com/alireza0/s-ui/database/model"
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

	log.Printf("Starting pure Go UDP tunnel '%s' (Mode: %s)", cfg.Name, cfg.Mode)

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

// runTunnel contains the core logic for the pure Go udp2raw implementation.
func (s *UdpTunnelService) runTunnel(ctx context.Context, cfg *model.UdpTunnelConfig) error {
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
	// The DSCP value is the top 6 bits of the 8-bit ToS field.
	// DSCP = ToS >> 2. So, ToS = DSCP << 2.
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
				return err
			}
			payload := buffer[:n]

			// Craft and send the FakeTCP packet
			err = sendFakeTCPPacket(fd, destIP, destPort, payload, tos)
			if err != nil {
				log.Printf("Failed to send FakeTCP packet: %v", err)
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
		Id:       12345, // Should be randomized
		Flags:    layers.IPv4DontFragment,
		TTL:      64,
		Protocol: layers.IPProtocolTCP,
		SrcIP:    srcIP,
		DstIP:    destIP,
	}
	tcpLayer := &layers.TCP{
		SrcPort: layers.TCPPort(12345), // Should be randomized
		DstPort: layers.TCPPort(destPort),
		SYN:     true, // This makes it "FakeTCP"
		Window:  14600,
		Seq:     1105024978, // Should be randomized
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
