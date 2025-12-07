package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"os/exec" // Added os/exec import
	"sync"
	"time" // Added time import for Cmd.Stop

	"github.com/igor04091968/sing-chisel-tel/database"
	"github.com/igor04091968/sing-chisel-tel/database/model"
	"gorm.io/gorm"
)

// MTProtoService handles the business logic for MTProto Proxy.
type MTProtoService struct {
	db             *gorm.DB
	activeProxies  map[uint]context.CancelFunc
	mu             sync.Mutex
	// External process management
	cmdMap         map[uint]*CmdWithContext // Map to store running commands
}

// CmdWithContext holds the command and its cancel function
type CmdWithContext struct {
	Cmd    *Cmd
	Cancel context.CancelFunc
}

// Cmd is a wrapper around os/exec.Cmd that supports context cancellation.
type Cmd struct {
	*exec.Cmd // Corrected embedding
	cancel context.CancelFunc
}

// NewCmd creates a new Cmd instance.
func NewCmd(ctx context.Context, name string, arg ...string) *Cmd {
	cmdCtx, cancel := context.WithCancel(ctx)
	cmd := exec.CommandContext(cmdCtx, name, arg...)
	return &Cmd{
		Cmd:    cmd,
		cancel: cancel,
	}
}

// Start starts the command.
func (c *Cmd) Start() error {
	return c.Cmd.Start()
}

// Wait waits for the command to complete.
func (c *Cmd) Wait() error {
	return c.Cmd.Wait()
}

// Stop attempts to stop the command gracefully, then forcefully.
func (c *Cmd) Stop() error {
	c.cancel() // Signal context cancellation
	// Give it a moment to exit gracefully
	time.Sleep(100 * time.Millisecond)
	if c.Process != nil && c.ProcessState == nil { // If process is still running
		return c.Process.Kill() // Force kill
	}
	return nil
}

// NewMTProtoService creates a new instance of MTProtoService.
func NewMTProtoService() *MTProtoService {
	return &MTProtoService{
		db:            database.GetDB(),
		activeProxies: make(map[uint]context.CancelFunc),
		cmdMap:        make(map[uint]*CmdWithContext),
	}
}

// StartMTProtoProxy starts an MTProto Proxy instance as an external process.
func (s *MTProtoService) StartMTProtoProxy(cfg *model.MTProtoProxyConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.cmdMap[cfg.ID]; exists {
		return fmt.Errorf("MTProto Proxy '%s' is already running", cfg.Name)
	}

	// Path to the mtg binary. Assume it's in PATH or current directory for now.
	// In a real deployment, you might want to specify a full path or download it.
	mtgBinary := "mtg"

	// Construct command-line arguments for mtg
	args := []string{
		"--bind-to", fmt.Sprintf("0.0.0.0:%d", cfg.ListenPort),
		"--secret", cfg.Secret,
	}
	if cfg.AdTag != "" {
		args = append(args, "--ad-tag", cfg.AdTag)
	}
	// Add other mtg options as needed, e.g., --prefer-ipv4, --doh-ip, --concurrency

	ctx, cancel := context.WithCancel(context.Background())
	cmd := NewCmd(ctx, mtgBinary, args...)

	s.cmdMap[cfg.ID] = &CmdWithContext{
		Cmd:    cmd,
		Cancel: cancel,
	}

	log.Printf("Starting external MTProto Proxy '%s' with command: %s %v", cfg.Name, mtgBinary, args)
	if err := cmd.Start(); err != nil {
		cancel()
		delete(s.cmdMap, cfg.ID)
		return fmt.Errorf("failed to start external MTProto Proxy '%s': %w", cfg.Name, err)
	}

	go func() {
		err := cmd.Wait() // Wait for the process to exit
		if err != nil && err.Error() != "signal: killed" && err != context.Canceled {
			log.Printf("External MTProto Proxy '%s' exited with error: %v", cfg.Name, err)
		} else {
			log.Printf("External MTProto Proxy '%s' stopped.", cfg.Name)
		}

		s.mu.Lock()
		defer s.mu.Unlock()
		if runningCmd, exists := s.cmdMap[cfg.ID]; exists && runningCmd.Cmd == cmd {
			delete(s.cmdMap, cfg.ID)
			// Update DB status
			var dbConfig model.MTProtoProxyConfig
			goroutineDB := database.GetDB()
			if goroutineDB.First(&dbConfig, cfg.ID).Error == nil {
				if dbConfig.Status == "up" {
					dbConfig.Status = "down"
					goroutineDB.Save(&dbConfig)
				}
			}
		}
	}()

	// Update DB status
	cfg.Status = "up"
	if err := s.db.Save(cfg).Error; err != nil {
		cancel() // Stop the external process if DB update fails
		_ = cmd.Process.Kill()
		delete(s.cmdMap, cfg.ID)
		return fmt.Errorf("failed to update MTProto Proxy status in DB: %w", err)
	}

	return nil
}

// StopMTProtoProxy stops a running MTProto Proxy instance.
func (s *MTProtoService) StopMTProtoProxy(id uint) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	cmdCtx, exists := s.cmdMap[id]
	if !exists {
		// If not in cmdMap, check DB status and update if necessary
		var dbConfig model.MTProtoProxyConfig
		if s.db.First(&dbConfig, id).Error == nil && dbConfig.Status == "up" {
			dbConfig.Status = "down"
			s.db.Save(&dbConfig)
		}
		return fmt.Errorf("MTProto Proxy with ID %d is not running", id)
	}

	cmdCtx.Cancel() // Signal the context to cancel the command
	// The goroutine started in StartMTProtoProxy will handle cleanup

	// Update DB status
	var dbConfig model.MTProtoProxyConfig
	if err := s.db.First(&dbConfig, id).Error; err != nil {
		return fmt.Errorf("failed to find MTProto Proxy with ID %d in DB: %w", id, err)
	}
	dbConfig.Status = "down"
	if err := s.db.Save(&dbConfig).Error; err != nil {
		return fmt.Errorf("failed to update MTProto Proxy status in DB: %w", err)
	}

	return nil
}

// GetMTProtoProxyByName retrieves a single MTProto Proxy configuration by its name.
func (s *MTProtoService) GetMTProtoProxyByName(name string) (*model.MTProtoProxyConfig, error) {
	var config model.MTProtoProxyConfig
	err := s.db.Where("name = ?", name).First(&config).Error
	return &config, err
}

// GetAllMTProtoProxies retrieves all MTProto Proxy configurations from the database.
func (s *MTProtoService) GetAllMTProtoProxies() ([]model.MTProtoProxyConfig, error) {
	var configs []model.MTProtoProxyConfig
	err := s.db.Find(&configs).Error
	return configs, err
}

// GetMTProtoProxy retrieves a single MTProto Proxy configuration by its ID.
func (s *MTProtoService) GetMTProtoProxy(id uint) (*model.MTProtoProxyConfig, error) {
	var config model.MTProtoProxyConfig
	err := s.db.First(&config, id).Error
	return &config, err
}

// CreateMTProtoProxy saves a new MTProto Proxy configuration to the database.
func (s *MTProtoService) CreateMTProtoProxy(config *model.MTProtoProxyConfig) error {
	return s.db.Create(config).Error
}

// UpdateMTProtoProxy updates an existing MTProto Proxy configuration in the database.
func (s *MTProtoService) UpdateMTProtoProxy(config *model.MTProtoProxyConfig) error {
	return s.db.Save(config).Error
}

// DeleteMTProtoProxy deletes an MTProto Proxy configuration from the database.
func (s *MTProtoService) DeleteMTProtoProxy(id uint) error {
	// Ensure proxy is stopped before deleting
	_ = s.StopMTProtoProxy(id) // Ignore error if not running

	return s.db.Delete(&model.MTProtoProxyConfig{}, id).Error
}

// GenerateMTProtoSecret generates a random 32-byte hex string suitable for an MTProto secret.
func GenerateMTProtoSecret() (string, error) {
	b := make([]byte, 16) // 16 bytes for a 32-char hex string
	_, err := rand.Read(b)
	if err != nil {
		return "", fmt.Errorf("failed to read random bytes: %w", err)
	}
	return hex.EncodeToString(b), nil
}