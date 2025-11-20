package service

import (
	"fmt"
	"log"
	"os/exec"
	"strconv"
	"sync"

	"github.com/alireza0/s-ui/database"
	"github.com/alireza0/s-ui/database/model"
	"gorm.io/gorm"
)

// UdpTunnelService manages UDP tunnels with udp2raw features.
type UdpTunnelService struct {
	db             *gorm.DB
	runningTunnels map[uint]*exec.Cmd
	mu             sync.Mutex
}

// NewUdpTunnelService creates a new UdpTunnelService
func NewUdpTunnelService(db *gorm.DB) *UdpTunnelService {
	return &UdpTunnelService{
		db:             db,
		runningTunnels: make(map[uint]*exec.Cmd),
	}
}

// StartUdpTunnel starts a UDP tunnel based on the provided configuration.
func (s *UdpTunnelService) StartUdpTunnel(cfg *model.UdpTunnelConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.runningTunnels[cfg.ID]; ok {
		return fmt.Errorf("UDP tunnel %s is already running", cfg.Name)
	}

	// Use the binary found in the project directory
	binaryPath := "./udp2raw"

	var args []string
	// A simple heuristic: if RemoteAddress is set, it's a client. Otherwise, it's a server.
	if cfg.RemoteAddress != "" {
		// Client mode
		args = append(args, "-c")
		// For client, ListenPort is the local port for the tunnel entry
		args = append(args, "-l", "127.0.0.1:"+strconv.Itoa(cfg.ListenPort))
		args = append(args, "-r", cfg.RemoteAddress)
	} else {
		// Server mode
		args = append(args, "-s")
		// For server, ListenPort is the public port udp2raw listens on
		args = append(args, "-l", "0.0.0.0:"+strconv.Itoa(cfg.ListenPort))
		// The model is missing a forwarding address for server mode.
		// This is a critical flaw in the current design.
		// We will log a fatal error because a server without a target is useless.
		return fmt.Errorf("cannot start udp2raw in server mode for tunnel '%s': RemoteAddress (forwarding address) is not set", cfg.Name)
	}

	if cfg.Mode != "" {
		args = append(args, "--raw-mode", cfg.Mode)
	}

	// Using a placeholder key as the model doesn't have a field for it.
	args = append(args, "-k", "your_secret_key") // IMPORTANT: This should be configurable

	if cfg.InterfaceName != "" {
		args = append(args, "--lower-level", cfg.InterfaceName)
	}
	
	// Add other flags based on the model if they were implemented
	// e.g., --dscp, --vlan-id, etc.

	cmd := exec.Command(binaryPath, args...)
	log.Printf("Starting UDP tunnel '%s' with command: %s %v", cfg.Name, binaryPath, args)

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start udp2raw process for tunnel %s: %w", cfg.Name, err)
	}

	cfg.ProcessID = cmd.Process.Pid
	cfg.Status = "running"
	if err := s.db.Save(cfg).Error; err != nil {
		_ = cmd.Process.Kill()
		return fmt.Errorf("failed to update tunnel status in DB: %w", err)
	}

	s.runningTunnels[cfg.ID] = cmd

	go func() {
		err := cmd.Wait()
		log.Printf("UDP tunnel '%s' (PID: %d) exited. Error: %v", cfg.Name, cfg.ProcessID, err)
		s.mu.Lock()
		defer s.mu.Unlock()
		
		if runningCmd, ok := s.runningTunnels[cfg.ID]; ok && runningCmd.Process.Pid == cfg.ProcessID {
			delete(s.runningTunnels, cfg.ID)
			var dbCfg model.UdpTunnelConfig
			// Use a new DB session for the goroutine
			goroutineDB := database.GetDB()
			if err := goroutineDB.First(&dbCfg, cfg.ID).Error; err == nil {
				if dbCfg.Status == "running" {
					dbCfg.Status = "stopped"
					dbCfg.ProcessID = 0
					database.GetDB().Save(&dbCfg)
				}
			}
		}
	}()

	return nil
}

// StopUdpTunnel stops a running UDP tunnel.
func (s *UdpTunnelService) StopUdpTunnel(id uint) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	cmd, ok := s.runningTunnels[id]
	if !ok {
		var cfg model.UdpTunnelConfig
		if err := s.db.First(&cfg, id).Error; err == nil && cfg.Status == "running" {
			cfg.Status = "stopped"
			cfg.ProcessID = 0
			s.db.Save(&cfg)
		}
		return fmt.Errorf("UDP tunnel with ID %d is not running in memory", id)
	}

	if cmd.Process == nil {
		delete(s.runningTunnels, id)
		return fmt.Errorf("process for tunnel ID %d not found", id)
	}

	log.Printf("Stopping UDP tunnel with PID: %d", cmd.Process.Pid)
	if err := cmd.Process.Kill(); err != nil {
		log.Printf("Failed to kill process %d: %v", cmd.Process.Pid, err)
	}

	delete(s.runningTunnels, id)

	var cfg model.UdpTunnelConfig
	if err := s.db.First(&cfg, id).Error; err != nil {
		return fmt.Errorf("failed to find tunnel with ID %d in DB after stopping: %w", id, err)
	}
	cfg.Status = "stopped"
	cfg.ProcessID = 0
	return s.db.Save(&cfg).Error
}

// GetAllUdpTunnels retrieves all UDP tunnel configurations from the database.
func (s *UdpTunnelService) GetAllUdpTunnels() ([]model.UdpTunnelConfig, error) {
	var tunnels []model.UdpTunnelConfig
	err := s.db.Find(&tunnels).Error
	return tunnels, err
}

// GetUdpTunnelByID retrieves a single UDP tunnel configuration by ID.
func (s *UdpTunnelService) GetUdpTunnelByID(id uint) (*model.UdpTunnelConfig, error) {
	var cfg model.UdpTunnelConfig
	err := s.db.First(&cfg, id).Error
	return &cfg, err
}

// GetUdpTunnelByName retrieves a single UdpTunnelConfig by its name.
func (s *UdpTunnelService) GetUdpTunnelByName(name string) (*model.UdpTunnelConfig, error) {
	var config model.UdpTunnelConfig
	err := s.db.Where("name = ?", name).First(&config).Error
	return &config, err
}

// CreateUdpTunnel saves a new UDP tunnel configuration to the database.
func (s *UdpTunnelService) CreateUdpTunnel(cfg *model.UdpTunnelConfig) error {
	return s.db.Create(cfg).Error
}

// UpdateUdpTunnel updates an existing UDP tunnel configuration in the database.
func (s *UdpTunnelService) UpdateUdpTunnel(cfg *model.UdpTunnelConfig) error {
	return s.db.Save(cfg).Error
}

// DeleteUdpTunnel deletes a UDP tunnel configuration from the database.
func (s *UdpTunnelService) DeleteUdpTunnel(id uint) error {
	if err := s.StopUdpTunnel(id); err != nil {
		log.Printf("Tunnel %d was not running, deleting from DB.", id)
	}
	return s.db.Delete(&model.UdpTunnelConfig{}, id).Error
}

// AutoStartUdpTunnels starts all tunnels marked as "running" on application startup.
func (s *UdpTunnelService) AutoStartUdpTunnels() {
	var tunnels []model.UdpTunnelConfig
	if err := s.db.Where("status = ?", "running").Find(&tunnels).Error; err != nil {
		fmt.Printf("Error retrieving UDP tunnels for autostart: %v\n", err)
		return
	}

	for i := range tunnels {
		log.Printf("Autostarting UDP tunnel: %s", tunnels[i].Name)
		// Use a local copy of the tunnel config for the goroutine
		tunnelToStart := tunnels[i]
		if err := s.StartUdpTunnel(&tunnelToStart); err != nil {
			log.Printf("Error autostarting UDP tunnel %s: %v", tunnelToStart.Name, err)
		}
	}
}
