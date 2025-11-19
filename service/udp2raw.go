package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	// "strings" // Removed unused import
	"sync"
	"syscall"
	"time"

	"github.com/alireza0/s-ui/database/model"
	"github.com/alireza0/s-ui/logger"
	"github.com/alireza0/s-ui/util" // Added import
	"gorm.io/gorm"
)

// Udp2rawService manages the lifecycle of goudp2raw tunnels.
type Udp2rawService struct {
	db *gorm.DB
	// Map to store running tunnel contexts for graceful shutdown
	runningTunnels map[uint]*runningTunnel
	mu             sync.Mutex
}

type runningTunnel struct {
	cancel context.CancelFunc
	cmd    *exec.Cmd
}

// NewUdp2rawService creates a new Udp2rawService.
func NewUdp2rawService(db *gorm.DB) *Udp2rawService {
	return &Udp2rawService{
		db:             db,
		runningTunnels: make(map[uint]*runningTunnel),
	}
}

// Save creates or updates a Udp2rawConfig in the database.
func (s *Udp2rawService) Save(config *model.Udp2rawConfig) error {
	if config.ID == 0 {
		return s.db.Create(config).Error
	}
	return s.db.Save(config).Error
}

// Get retrieves a Udp2rawConfig by name.
func (s *Udp2rawService) Get(name string) (*model.Udp2rawConfig, error) {
	var config model.Udp2rawConfig
	err := s.db.Where("name = ?", name).First(&config).Error
	return &config, err
}

// GetByID retrieves a Udp2rawConfig by ID.
func (s *Udp2rawService) GetByID(id uint) (*model.Udp2rawConfig, error) {
	var config model.Udp2rawConfig
	err := s.db.First(&config, id).Error
	return &config, err
}

// GetAll retrieves all Udp2rawConfig entries.
func (s *Udp2rawService) GetAll() ([]model.Udp2rawConfig, error) {
	var configs []model.Udp2rawConfig
	err := s.db.Find(&configs).Error
	return configs, err
}

// Delete deletes a Udp2rawConfig by name.
func (s *Udp2rawService) Delete(name string) error {
	config, err := s.Get(name)
	if err != nil {
		return err
	}
	if config.PID != 0 {
		s.Stop(config) // Stop the running process first
	}
	return s.db.Delete(config).Error
}

// Start starts a goudp2raw tunnel process.
func (s *Udp2rawService) Start(config *model.Udp2rawConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if config.PID != 0 {
		return errors.New("tunnel is already running")
	}

	var binaryName string
	var args []string

	switch config.Mode {
	case "client":
		binaryName = "./goudp2raw/goudp2raw-client"
		args = []string{
			"-l", config.LocalAddr,
			"-r", config.RemoteAddr,
			"-k", config.Key,
		}
	case "server":
		binaryName = "./goudp2raw/goudp2raw-server"
		args = []string{
			"-l", config.LocalAddr,
			"-r", config.RemoteAddr,
			"-k", config.Key,
		}
	default:
		return fmt.Errorf("unsupported mode: %s", config.Mode)
	}

	// Add DSCP if specified
	if config.DSCP > 0 {
		args = append(args, "-dscp", strconv.Itoa(config.DSCP))
	}

	// Add extra args from JSON
	if len(config.Args) > 2 { // Check if it's not just "{}"
		var extraArgs map[string]string
		if err := json.Unmarshal(config.Args, &extraArgs); err != nil {
			logger.Error("Failed to unmarshal extra args for udp2raw config %s: %v", config.Name, err)
		} else {
			for k, v := range extraArgs {
				args = append(args, k, v)
			}
		}
	}

	// Check if binary exists
	if _, err := os.Stat(binaryName); os.IsNotExist(err) {
		return fmt.Errorf("goudp2raw binary not found at %s. Please build it first.", binaryName)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, binaryName, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true} // Detach from parent process group

	// Capture stderr for logging
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		return fmt.Errorf("failed to get stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		cancel()
		return fmt.Errorf("failed to start goudp2raw process: %w", err)
	}

	// Read stderr in a goroutine
	go func() {
		scanner := util.NewNewLineScanner(stderrPipe) // Updated call
		for scanner.Scan() {
			logger.Error("[GOUDP2RAW-%s] %s", config.Name, scanner.Text())
		}
		if err := scanner.Err(); err != nil {
			logger.Error("[GOUDP2RAW-%s] Error reading stderr: %v", config.Name, err)
		}
	}()

	// Monitor process in a goroutine
	go func() {
		err := cmd.Wait()
		s.mu.Lock()
		defer s.mu.Unlock()
		delete(s.runningTunnels, config.ID) // Remove from running map

		// Update database status if process exited unexpectedly
		var currentConfig model.Udp2rawConfig
		if s.db.First(&currentConfig, config.ID).Error == nil && currentConfig.PID != 0 {
			currentConfig.PID = 0
			currentConfig.Status = "stopped"
			s.db.Save(&currentConfig)
			logger.Info("[GOUDP2RAW-%s] Process exited. Status updated to stopped. Error: %v", config.Name, err)
		} else {
			logger.Info("[GOUDP2RAW-%s] Process exited. Error: %v", config.Name, err)
		}
		cancel() // Ensure context is cancelled
	}()

	config.PID = cmd.Process.Pid
	config.Status = "running"
	s.db.Save(config)

	s.runningTunnels[config.ID] = &runningTunnel{
		cancel: cancel,
		cmd:    cmd,
	}

	logger.Info("[GOUDP2RAW-%s] Started process with PID %d", config.Name, config.PID)
	return nil
}

// Stop stops a running goudp2raw tunnel process.
func (s *Udp2rawService) Stop(config *model.Udp2rawConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if config.PID == 0 {
		return errors.New("tunnel is not running")
	}

	rt, ok := s.runningTunnels[config.ID]
	if !ok {
		// Process not in our map, but PID is set. Try to kill by PID.
		process, err := os.FindProcess(config.PID)
		if err == nil {
			logger.Info("[GOUDP2RAW-%s] Found process %d not in map, attempting to kill.", config.Name, config.PID)
			if err := process.Kill(); err != nil {
				logger.Error("[GOUDP2RAW-%s] Failed to kill process %d: %v", config.Name, config.PID, err)
			}
		}
		config.PID = 0
		config.Status = "stopped"
		s.db.Save(config)
		return errors.New("tunnel process not found in running map, but PID was set. Status reset.")
	}

	// Use context cancellation for graceful shutdown
	rt.cancel()
	// Give it a moment to terminate
	select {
	case <-time.After(5 * time.Second):
		if rt.cmd.Process != nil {
			logger.Warning("[GOUDP2RAW-%s] Process %d did not terminate gracefully, forcing kill.", config.Name, rt.cmd.Process.Pid)
			if err := rt.cmd.Process.Kill(); err != nil {
				logger.Error("[GOUDP2RAW-%s] Failed to force kill process %d: %v", config.Name, rt.cmd.Process.Pid, err)
			}
		}
	case <-time.After(100 * time.Millisecond): // Wait for the goroutine to clean up
	}

	delete(s.runningTunnels, config.ID)
	config.PID = 0
	config.Status = "stopped"
	s.db.Save(config)
	logger.Info("[GOUDP2RAW-%s] Stopped process %d", config.Name, config.PID)
	return nil
}

// StopAllActiveUdp2rawServices stops all running goudp2raw tunnels.
func (s *Udp2rawService) StopAllActiveUdp2rawServices() {
	configs, err := s.GetAll()
	if err != nil {
		logger.Error("Failed to get all udp2raw configs for stopping: %v", err)
		return
	}
	for _, config := range configs {
		if config.PID != 0 {
			s.Stop(&config) // Pass a copy
		}
	}
}

// StartAllUdp2rawServices starts all configured goudp2raw tunnels.
func (s *Udp2rawService) StartAllUdp2rawServices() {
	configs, err := s.GetAll()
	if err != nil {
		logger.Error("Failed to get all udp2raw configs for starting: %v", err)
		return
	}
	for _, config := range configs {
		if config.Status == "running" && config.PID == 0 { // Only start if status is running but PID is 0 (e.g., after restart)
			logger.Info("[GOUDP2RAW-%s] Attempting to restart previously running tunnel.", config.Name)
			s.Start(&config) // Pass a copy
		} else if config.Status == "running" && config.PID != 0 {
			// Check if process is actually running
			if _, err := os.FindProcess(config.PID); err != nil {
				logger.Warning("[GOUDP2RAW-%s] PID %d found in DB but process not running. Resetting status.", config.Name, config.PID)
				config.PID = 0
				config.Status = "stopped"
				s.db.Save(&config)
			} else {
				logger.Info("[GOUDP2RAW-%s] Process with PID %d is already running.", config.Name, config.PID)
				s.mu.Lock()
				s.runningTunnels[config.ID] = &runningTunnel{
					cmd: exec.Command("kill", "-0", strconv.Itoa(config.PID)), // Dummy command for context
				}
				s.mu.Unlock()
			}
		}
	}
}

// ResetPIDs resets all PID entries in the database to 0.
// This is called on application startup to ensure all tunnels are re-evaluated.
func (s *Udp2rawService) ResetPIDs() error {
	return s.db.Model(&model.Udp2rawConfig{}).Where("pid != ?", 0).Update("pid", 0).Error
}
