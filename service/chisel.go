package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/igor04091968/sing-chisel-tel/database"
	"github.com/igor04091968/sing-chisel-tel/database/model"
	chclient "github.com/jpillora/chisel/client"
	chserver "github.com/jpillora/chisel/server"
)

type ChiselService struct {
	activeServices map[uint]context.CancelFunc
	mu             sync.Mutex
}

func NewChiselService() *ChiselService {
	return &ChiselService{
		activeServices: make(map[uint]context.CancelFunc),
	}
}

func (s *ChiselService) GetAllChiselConfigs() ([]model.ChiselConfig, error) {
	db := database.GetDB()
	var configs []model.ChiselConfig
	err := db.Find(&configs).Error
	return configs, err
}

func (s *ChiselService) GetChiselConfig(id uint) (*model.ChiselConfig, error) {
	db := database.GetDB()
	var config model.ChiselConfig
	err := db.First(&config, id).Error
	return &config, err
}

func (s *ChiselService) GetChiselConfigByName(name string) (*model.ChiselConfig, error) {
	db := database.GetDB()
	var config model.ChiselConfig
	err := db.Where("name = ?", name).First(&config).Error
	return &config, err
}

func (s *ChiselService) CreateChiselConfig(config *model.ChiselConfig) error {
	db := database.GetDB()
	return db.Create(config).Error
}

func (s *ChiselService) UpdateChiselConfig(config *model.ChiselConfig) error {
	db := database.GetDB()
	return db.Save(config).Error
}

func (s *ChiselService) DeleteChiselConfig(id uint) error {
	db := database.GetDB()
	// Use Unscoped() to perform a permanent hard delete, bypassing the soft delete mechanism.
	return db.Unscoped().Delete(&model.ChiselConfig{}, id).Error
}

func (s *ChiselService) Save(act string, data json.RawMessage) error {
	db := database.GetDB()
	var err error
	switch act {
	case "new", "update":
		var config model.ChiselConfig
		err = json.Unmarshal(data, &config)
		if err != nil {
			return err
		}
		if act == "new" {
			err = db.Create(&config).Error
		} else {
			err = db.Save(&config).Error
		}
	case "del":
		var id uint
		err = json.Unmarshal(data, &id)
		if err != nil {
			return err
		}
		var config model.ChiselConfig
		err = db.First(&config, id).Error
		if err != nil {
			return err
		}
		s.mu.Lock()
		cancel, exists := s.activeServices[config.ID]
		s.mu.Unlock()
		if exists {
			cancel()
			s.mu.Lock()
			delete(s.activeServices, config.ID)
			s.mu.Unlock()
		}
		err = db.Delete(&model.ChiselConfig{}, id).Error

	default:
		return fmt.Errorf("unknown action: %s", act)
	}
	return err
}

// GetActiveChiselConfigIDs returns a slice of IDs for currently active Chisel services.
func (s *ChiselService) GetActiveChiselConfigIDs() []uint {
	s.mu.Lock()
	defer s.mu.Unlock()

	ids := make([]uint, 0, len(s.activeServices))
	for id := range s.activeServices {
		ids = append(ids, id)
	}
	return ids
}

// StopAllActiveChiselServices iterates through all active Chisel services and stops them.
func (s *ChiselService) StopAllActiveChiselServices() {
	s.mu.Lock()
	// Create a copy of activeServices keys to avoid deadlock during iteration and modification
	idsToStop := make([]uint, 0, len(s.activeServices))
	for id := range s.activeServices {
		idsToStop = append(idsToStop, id)
	}
	s.mu.Unlock() // Unlock before calling StopChisel to avoid deadlock

	for _, id := range idsToStop {
		cfg, err := s.GetChiselConfig(id)
		if err != nil {
			log.Printf("ChiselService: Error getting Chisel config for ID %d during StopAllActiveChiselServices: %v", id, err)
			continue
		}
		if err := s.StopChisel(cfg); err != nil {
			log.Printf("ChiselService: Error stopping Chisel service '%s' (ID: %d) during StopAllActiveChiselServices: %v", cfg.Name, cfg.ID, err)
		} else {
			log.Printf("ChiselService: Chisel service '%s' (ID: %d) stopped by StopAllActiveChiselServices.", cfg.Name, cfg.ID)
		}
	}
}

func (s *ChiselService) StartChisel(config *model.ChiselConfig) error {
	db := database.GetDB()
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.activeServices[config.ID]; exists {
		return fmt.Errorf("service '%s' is already running", config.Name)
	}

	ctx, cancel := context.WithCancel(context.Background())
	s.activeServices[config.ID] = cancel

	var (
		chiselClient *chclient.Client
		chiselServer *chserver.Server
		err          error
	)
	args := strings.Fields(config.Args)

	if config.Mode == "client" {
		remotes := []string{}
		auth := ""
		skipVerify := false

		i := 0
		for i < len(args) {
			arg := args[i]
			if arg == "--auth" && i+1 < len(args) {
				auth = args[i+1]
				i += 2
			} else if arg == "--tls-skip-verify" || arg == "--tls" {
				skipVerify = true
				i++
			} else {
				remotes = append(remotes, arg)
				i++
			}
		}

		serverURL := fmt.Sprintf("%s:%d", config.ServerAddress, config.ServerPort)
		if skipVerify {
			serverURL = "https://" + serverURL
		}

		clientConfig := &chclient.Config{
			Remotes:   remotes,
			Auth:      auth,
			Server:    serverURL,
			KeepAlive: 25 * time.Second,
			Headers:   http.Header{},
			TLS: chclient.TLSConfig{
				SkipVerify: skipVerify,
				ServerName: config.ServerAddress,
			},
		}

		chiselClient, err = chclient.NewClient(clientConfig)
		if err != nil {
			cancel()
			delete(s.activeServices, config.ID)
			return fmt.Errorf("failed to create Chisel client '%s': %w", config.Name, err)
		}
	} else { // server
		serverConfig := &chserver.Config{
			Reverse:   false,
			KeepAlive: 25 * time.Second,
		}
		for i, arg := range args {
			switch arg {
			case "--reverse":
				serverConfig.Reverse = true
			case "--auth":
				if i+1 < len(args) {
					serverConfig.Auth = args[i+1]
				}
			}
		}

		chiselServer, err = chserver.NewServer(serverConfig)
		if err != nil {
			cancel()
			delete(s.activeServices, config.ID)
			return fmt.Errorf("failed to create Chisel server '%s': %w", config.Name, err)
		}
	}

	// If we reached here, client/server was successfully created.
	// Now update PID in DB and then launch the goroutine.
	config.PID = 1
	log.Printf("ChiselService: StartChisel: Attempting to save PID %d for config '%s' (ID: %d)", config.PID, config.Name, config.ID)
	if err := db.Save(config).Error; err != nil {
		cancel()
		delete(s.activeServices, config.ID)
		return fmt.Errorf("failed to update Chisel config PID in DB for '%s': %w", config.Name, err)
	}
	log.Printf("ChiselService: StartChisel: Successfully saved PID %d for config '%s' (ID: %d)", config.PID, config.Name, config.ID)

	go func(cfg *model.ChiselConfig, client *chclient.Client, server *chserver.Server) {
		defer func() {
			s.mu.Lock()
			delete(s.activeServices, cfg.ID)
			s.mu.Unlock()

			var dbConfig model.ChiselConfig
			goroutineDB := database.GetDB()
			if goroutineDB.First(&dbConfig, cfg.ID).Error == nil {
				if dbConfig.PID != 0 {
					dbConfig.PID = 0
					goroutineDB.Save(&dbConfig)
					log.Printf("ChiselService: Goroutine defer: Reset PID to 0 for config '%s' (ID: %d)", cfg.Name, cfg.ID)
				}
			}
			log.Printf("Chisel service '%s' stopped.", cfg.Name)
		}()

		        var runErr error
				if cfg.Mode == "client" {
					log.Printf("ChiselService: Goroutine: Attempting to start Chisel client '%s' (ID: %d)", cfg.Name, cfg.ID)
					runErr = client.Start(ctx)
					log.Printf("ChiselService: Goroutine: Chisel client '%s' (ID: %d) Start() returned: %v", cfg.Name, cfg.ID, runErr)
				} else { // server
					host := "0.0.0.0"
					port := fmt.Sprintf("%d", cfg.ListenPort)
					log.Printf("ChiselService: Goroutine: Attempting to start Chisel server '%s' (ID: %d) on %s:%s", cfg.Name, cfg.ID, host, port)
					runErr = server.StartContext(ctx, host, port)
					log.Printf("ChiselService: Goroutine: Chisel server '%s' (ID: %d) StartContext() returned: %v", cfg.Name, cfg.ID, runErr)
				}
		
				if runErr != nil && runErr != context.Canceled {
					log.Printf("Error running chisel service '%s': %v", cfg.Name, runErr)
				} else if runErr == nil { // If Start was successful, wait for context cancellation
					log.Printf("ChiselService: Goroutine: Chisel service '%s' (ID: %d) started successfully, waiting for context cancellation.", cfg.Name, cfg.ID)
					<-ctx.Done() // Block until context is cancelled
					log.Printf("ChiselService: Goroutine: Context cancelled for '%s' (ID: %d).", cfg.Name, cfg.ID)
				}	}(config, chiselClient, chiselServer) // Pass client/server instance to the goroutine

	return nil
}

func (s *ChiselService) StopChisel(config *model.ChiselConfig) error {
	db := database.GetDB()
	s.mu.Lock()
	defer s.mu.Unlock()

	// Stop the service if it's in the active map
	if cancel, exists := s.activeServices[config.ID]; exists {
		cancel()
		delete(s.activeServices, config.ID)
		log.Printf("ChiselService: Cancelled and removed active service '%s' (ID: %d) from map.", config.Name, config.ID)
	}

	// Always ensure PID is 0 in the database, regardless of whether it was in the active map.
	// This handles cleanup of stale PIDs from previous crashes.
	if config.PID != 0 {
		config.PID = 0
		if err := db.Save(config).Error; err != nil {
			log.Printf("ChiselService: Error resetting PID to 0 for config '%s' (ID: %d) in DB: %v", config.Name, config.ID, err)
			return fmt.Errorf("failed to reset PID for '%s': %w", config.Name, err)
		}
		log.Printf("ChiselService: Successfully reset PID to 0 for config '%s' (ID: %d)", config.Name, config.ID)
	}

	return nil // Always return nil on success, even if it wasn't in the active map.
}