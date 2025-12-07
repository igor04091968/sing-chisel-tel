//go:build !skip_gost
// +build !skip_gost

package service

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"time"

	"github.com/igor04091968/sing-chisel-tel/database"
	"github.com/igor04091968/sing-chisel-tel/database/model"
)

// GostService provides embedded reverse tunnel functionality (no external binaries needed).
type GostService struct {
	mu     sync.Mutex
	ctxMap map[uint]context.CancelFunc
	lnMap  map[uint]net.Listener
}

// NewGostService creates a new GostService.
func NewGostService() *GostService {
	return &GostService{
		ctxMap: make(map[uint]context.CancelFunc),
		lnMap:  make(map[uint]net.Listener),
	}
}

// StartGost starts an embedded reverse tunnel based on configuration.
func (s *GostService) StartGost(cfg *model.GostConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.ctxMap[cfg.ID]; exists {
		return fmt.Errorf("gost '%s' is already running", cfg.Name)
	}

	// Create TCP listener on the configured port
	listenAddr := fmt.Sprintf("%s:%d", cfg.ListenAddress, cfg.ListenPort)
	ln, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", listenAddr, err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Determine target for reverse tunnel
	var targetAddr string
	if cfg.Mode == "client" && cfg.ServerAddress != "" && cfg.ServerPort > 0 {
		// Client mode: forward to remote server
		targetAddr = fmt.Sprintf("%s:%d", cfg.ServerAddress, cfg.ServerPort)
	} else if cfg.Args != "" {
		// Parse target from Args if provided (format: "host:port")
		targetAddr = cfg.Args
	} else {
		// Fallback: use loopback for server mode, or error for client
		if cfg.Mode == "client" {
			cancel()
			ln.Close()
			return fmt.Errorf("client mode requires ServerAddress:ServerPort or Args with target")
		}
		targetAddr = "127.0.0.1:8000" // default server passthrough
	}

	// Accept connections in background goroutine
	go func(listener net.Listener, id uint, name, target string) {
		log.Printf("gost tunnel '%s' (id=%d) [%s] started, listening on %s, forwarding to %s", 
			name, id, cfg.Mode, listenAddr, target)

		for {
			select {
			case <-ctx.Done():
				listener.Close()
				log.Printf("gost tunnel '%s' (id=%d) stopped", name, id)
				s.mu.Lock()
				defer s.mu.Unlock()
				delete(s.ctxMap, id)
				delete(s.lnMap, id)

				var dbCfg model.GostConfig
				gdb := database.GetDB()
				if gdb.First(&dbCfg, id).Error == nil {
					dbCfg.Status = "down"
					gdb.Save(&dbCfg)
				}
				return
			default:
				// Set accept deadline to allow periodic ctx.Done() checks
				listener.(*net.TCPListener).SetDeadline(time.Now().Add(1 * time.Second))
				conn, err := listener.Accept()
				if err != nil {
					if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
						continue
					}
					if err == context.Canceled {
						return
					}
					continue
				}
				// Handle reverse tunnel: forward to target
				go forwardConnection(conn, target, cfg.Name, id)
			}
		}
	}(ln, cfg.ID, cfg.Name, targetAddr)

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
		return fmt.Errorf("failed to update gost status in DB: %w", err)
	}

	return nil
}

// StopGost stops a running reverse tunnel by ID.
func (s *GostService) StopGost(id uint) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	cancel, ok := s.ctxMap[id]
	if !ok {
		db := database.GetDB()
		var cfg model.GostConfig
		if db.First(&cfg, id).Error == nil && cfg.Status == "up" {
			cfg.Status = "down"
			db.Save(&cfg)
		}
		return fmt.Errorf("gost with id %d is not running", id)
	}

	// Signal cancellation and close listener
	cancel()
	if ln, ok := s.lnMap[id]; ok {
		ln.Close()
	}

	delete(s.ctxMap, id)
	delete(s.lnMap, id)

	db := database.GetDB()
	var cfg model.GostConfig
	if db.First(&cfg, id).Error == nil {
		cfg.Status = "down"
		db.Save(&cfg)
	}

	return nil
}

// forwardConnection handles bidirectional forwarding between client and target server
func forwardConnection(clientConn net.Conn, targetAddr, tunnelName string, tunnelID uint) {
	defer clientConn.Close()

	// Connect to target
	targetConn, err := net.DialTimeout("tcp", targetAddr, 5*time.Second)
	if err != nil {
		log.Printf("gost tunnel '%s' (id=%d) failed to connect to target %s: %v", 
			tunnelName, tunnelID, targetAddr, err)
		return
	}
	defer targetConn.Close()

	// Bidirectional copy
	errChan := make(chan error, 2)
	go func() {
		_, err := io.Copy(targetConn, clientConn)
		errChan <- err
	}()
	go func() {
		_, err := io.Copy(clientConn, targetConn)
		errChan <- err
	}()

	// Wait for one direction to finish
	<-errChan
}

// GetAllGostConfigs retrieves all gost tunnel configurations from the database.
func (s *GostService) GetAllGostConfigs() ([]model.GostConfig, error) {
	var configs []model.GostConfig
	err := database.GetDB().Find(&configs).Error
	return configs, err
}

// GetGostConfigByName retrieves a gost configuration by name.
func (s *GostService) GetGostConfigByName(name string) (*model.GostConfig, error) {
	var config model.GostConfig
	err := database.GetDB().Where("name = ?", name).First(&config).Error
	return &config, err
}

// CreateGostConfig saves a new gost configuration to the database.
func (s *GostService) CreateGostConfig(cfg *model.GostConfig) error {
	return database.GetDB().Create(cfg).Error
}

// DeleteGostConfig deletes a gost configuration from the database.
func (s *GostService) DeleteGostConfig(id uint) error {
	_ = s.StopGost(id)
	return database.GetDB().Delete(&model.GostConfig{}, id).Error
}
