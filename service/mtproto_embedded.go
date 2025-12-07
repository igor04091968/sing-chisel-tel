package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"github.com/igor04091968/sing-chisel-tel/database"
	"github.com/igor04091968/sing-chisel-tel/database/model"
)

// MTProtoEmbeddedService provides embedded MTProto proxy support
type MTProtoEmbeddedService struct {
	mu     sync.Mutex
	ctxMap map[uint]context.CancelFunc
	lnMap  map[uint]net.Listener
}

// NewMTProtoEmbeddedService creates a new MTProto service instance
func NewMTProtoEmbeddedService() *MTProtoEmbeddedService {
	return &MTProtoEmbeddedService{
		ctxMap: make(map[uint]context.CancelFunc),
		lnMap:  make(map[uint]net.Listener),
	}
}

// StartMTProto starts an MTProto proxy tunnel
func (s *MTProtoEmbeddedService) StartMTProto(cfg *model.MTProtoProxyConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.ctxMap[cfg.ID]; exists {
		return fmt.Errorf("MTProto proxy '%s' is already running", cfg.Name)
	}

	// Validate secret (should be 32-byte hex string)
	if len(cfg.Secret) != 64 { // 32 bytes = 64 hex chars
		return fmt.Errorf("MTProto secret must be 32-byte hex string (64 chars), got %d", len(cfg.Secret))
	}

	secretBytes, err := hex.DecodeString(cfg.Secret)
	if err != nil {
		return fmt.Errorf("invalid MTProto secret hex: %w", err)
	}

	if len(secretBytes) != 32 {
		return fmt.Errorf("MTProto secret must be exactly 32 bytes")
	}

	// Create TCP listener on the configured port
	listenAddr := fmt.Sprintf("0.0.0.0:%d", cfg.ListenPort)
	ln, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", listenAddr, err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Accept connections in background goroutine
	go func(listener net.Listener, id uint, name string, secret []byte) {
		log.Printf("MTProto proxy '%s' (id=%d) started, listening on %s", name, id, listenAddr)

		for {
			select {
			case <-ctx.Done():
				listener.Close()
				log.Printf("MTProto proxy '%s' (id=%d) stopped", name, id)
				s.mu.Lock()
				defer s.mu.Unlock()
				delete(s.ctxMap, id)
				delete(s.lnMap, id)

				var dbCfg model.MTProtoProxyConfig
				gdb := database.GetDB()
				if gdb.First(&dbCfg, id).Error == nil {
					dbCfg.Status = "down"
					gdb.Save(&dbCfg)
				}
				return
			default:
				// Set accept deadline to allow periodic ctx.Done() checks
				listener.(*net.TCPListener).SetDeadline(time.Now().Add(1 * time.Second))
				clientConn, err := listener.Accept()
				if err != nil {
					if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
						continue
					}
					if err == context.Canceled {
						return
					}
					continue
				}

				// Handle MTProto connection in background
				go handleMTProtoConnection(clientConn, secret, name, id)
			}
		}
	}(ln, cfg.ID, cfg.Name, secretBytes)

	s.ctxMap[cfg.ID] = cancel
	s.lnMap[cfg.ID] = ln

	// Update DB status
	db := database.GetDB()
	cfg.Status = "up"
	if err := db.Save(cfg).Error; err != nil {
		cancel()
		ln.Close()
		delete(s.ctxMap, cfg.ID)
		delete(s.lnMap, cfg.ID)
		return fmt.Errorf("failed to update MTProto status in DB: %w", err)
	}

	return nil
}

// handleMTProtoConnection handles a single MTProto client connection
func handleMTProtoConnection(clientConn net.Conn, secret []byte, proxyName string, proxyID uint) {
	defer clientConn.Close()

	// MTProto protocol: Read initial packet and validate
	// Client sends: 1 byte (0xEF) + 4 bytes (length) + payload
	// We'll use a simple forwarding approach with secret validation

	// Read initial handshake
	handshake := make([]byte, 1024)
	clientConn.SetReadDeadline(time.Now().Add(5 * time.Second))
	n, err := clientConn.Read(handshake)
	if err != nil || n < 64 {
		log.Printf("MTProto proxy '%s' (id=%d): invalid handshake: %v", proxyName, proxyID, err)
		return
	}

	// Check if first byte is valid MTProto marker (0xEF for obfuscated2 or direct connection)
	// For now, we accept any connection and forward to Telegram servers
	// In production, validate the secret properly

	// Connect to Telegram MTProto server (core servers: 149.154.167.* on port 443)
	// Using one of Telegram's production servers
	targetAddr := "149.154.167.40:443"
	targetConn, err := net.DialTimeout("tcp", targetAddr, 5*time.Second)
	if err != nil {
		log.Printf("MTProto proxy '%s' (id=%d) failed to connect to Telegram: %v",
			proxyName, proxyID, targetAddr, err)
		return
	}
	defer targetConn.Close()

	// Send initial data to target
	if _, err := targetConn.Write(handshake[:n]); err != nil {
		log.Printf("MTProto proxy '%s' (id=%d) failed to write to Telegram: %v",
			proxyName, proxyID, err)
		return
	}

	// Bidirectional relay
	errChan := make(chan error, 2)
	go func() {
		_, err := relayData(targetConn, clientConn, "client->telegram")
		errChan <- err
	}()
	go func() {
		_, err := relayData(clientConn, targetConn, "telegram->client")
		errChan <- err
	}()

	// Wait for first error
	<-errChan
}

// relayData copies data between connections with buffering
func relayData(dst, src net.Conn, direction string) (int64, error) {
	buf := make([]byte, 32*1024) // 32KB buffer
	var total int64

	for {
		n, err := src.Read(buf)
		if n > 0 {
			if _, err := dst.Write(buf[:n]); err != nil {
				return total, err
			}
			total += int64(n)
		}
		if err != nil {
			return total, nil // EOF is expected
		}
	}
}

// StopMTProto stops an MTProto proxy tunnel
func (s *MTProtoEmbeddedService) StopMTProto(id uint) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	cancel, ok := s.ctxMap[id]
	if !ok {
		db := database.GetDB()
		var cfg model.MTProtoProxyConfig
		if db.First(&cfg, id).Error == nil && cfg.Status == "up" {
			cfg.Status = "down"
			db.Save(&cfg)
		}
		return fmt.Errorf("MTProto proxy with id %d is not running", id)
	}

	// Signal cancellation and close listener
	cancel()
	if ln, ok := s.lnMap[id]; ok {
		ln.Close()
	}

	delete(s.ctxMap, id)
	delete(s.lnMap, id)

	db := database.GetDB()
	var cfg model.MTProtoProxyConfig
	if db.First(&cfg, id).Error == nil {
		cfg.Status = "down"
		db.Save(&cfg)
	}

	return nil
}

// GetAllMTProtoConfigs retrieves all MTProto configurations
func (s *MTProtoEmbeddedService) GetAllMTProtoConfigs() ([]model.MTProtoProxyConfig, error) {
	var configs []model.MTProtoProxyConfig
	err := database.GetDB().Find(&configs).Error
	return configs, err
}

// GetMTProtoConfigByName retrieves MTProto config by name
func (s *MTProtoEmbeddedService) GetMTProtoConfigByName(name string) (*model.MTProtoProxyConfig, error) {
	var config model.MTProtoProxyConfig
	err := database.GetDB().Where("name = ?", name).First(&config).Error
	return &config, err
}

// CreateMTProtoConfig creates a new MTProto configuration
func (s *MTProtoEmbeddedService) CreateMTProtoConfig(cfg *model.MTProtoProxyConfig) error {
	// Generate random secret if not provided
	if cfg.Secret == "" {
		secret := make([]byte, 32)
		if _, err := rand.Read(secret); err != nil {
			return fmt.Errorf("failed to generate secret: %w", err)
		}
		cfg.Secret = hex.EncodeToString(secret)
	}
	return database.GetDB().Create(cfg).Error
}

// DeleteMTProtoConfig deletes an MTProto configuration
func (s *MTProtoEmbeddedService) DeleteMTProtoConfig(id uint) error {
	_ = s.StopMTProto(id)
	return database.GetDB().Delete(&model.MTProtoProxyConfig{}, id).Error
}

// UpdateMTProtoConfig updates an MTProto configuration
func (s *MTProtoEmbeddedService) UpdateMTProtoConfig(cfg *model.MTProtoProxyConfig) error {
	return database.GetDB().Save(cfg).Error
}
