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

	"github.com/alireza0/s-ui/database/model"
	chclient "github.com/jpillora/chisel/client"
	chserver "github.com/jpillora/chisel/server"
	"gorm.io/gorm"
)

type ChiselService struct {
	db             *gorm.DB
	activeServices map[uint]context.CancelFunc
	mu             sync.Mutex
}

func NewChiselService(db *gorm.DB) *ChiselService {
	return &ChiselService{
		db:             db,
		activeServices: make(map[uint]context.CancelFunc),
	}
}

func (s *ChiselService) GetAllChiselConfigs() ([]model.ChiselConfig, error) {
	var configs []model.ChiselConfig
	err := s.db.Find(&configs).Error
	return configs, err
}

func (s *ChiselService) GetChiselConfig(id uint) (*model.ChiselConfig, error) {
	var config model.ChiselConfig
	err := s.db.First(&config, id).Error
	return &config, err
}

func (s *ChiselService) GetChiselConfigByName(name string) (*model.ChiselConfig, error) {
	var config model.ChiselConfig
	err := s.db.Where("name = ?", name).First(&config).Error
	return &config, err
}

func (s *ChiselService) CreateChiselConfig(config *model.ChiselConfig) error {
	return s.db.Create(config).Error
}

func (s *ChiselService) UpdateChiselConfig(config *model.ChiselConfig) error {
	return s.db.Save(config).Error
}

func (s *ChiselService) DeleteChiselConfig(id uint) error {
	return s.db.Delete(&model.ChiselConfig{}, id).Error
}

func (s *ChiselService) Save(tx *gorm.DB, act string, data json.RawMessage) error {
	var err error
	switch act {
	case "new", "update":
		var config model.ChiselConfig
		err = json.Unmarshal(data, &config)
		if err != nil {
			return err
		}
		if act == "new" {
			err = tx.Create(&config).Error
		} else {
			err = tx.Save(&config).Error
		}
	case "del":
		var id uint
		err = json.Unmarshal(data, &id)
		if err != nil {
			return err
		}
		var config model.ChiselConfig
		err = tx.First(&config, id).Error
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
		err = tx.Delete(&model.ChiselConfig{}, id).Error

	default:
		return fmt.Errorf("unknown action: %s", act)
	}
	return err
}


func (s *ChiselService) StartChisel(config *model.ChiselConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.activeServices[config.ID]; exists {
		return fmt.Errorf("service '%s' is already running", config.Name)
	}

	ctx, cancel := context.WithCancel(context.Background())
	s.activeServices[config.ID] = cancel

	go func() {
		defer func() {
			s.mu.Lock()
			delete(s.activeServices, config.ID)
			s.mu.Unlock()

			var dbConfig model.ChiselConfig
			if s.db.First(&dbConfig, config.ID).Error == nil {
				if dbConfig.PID != 0 {
					dbConfig.PID = 0
					s.db.Save(&dbConfig)
				}
			}
			log.Printf("Chisel service '%s' stopped.", config.Name)
		}()

		var err error
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

			clientConfig := &chclient.Config{
				Remotes:   remotes,
				Auth:      auth,
				Server:    fmt.Sprintf("%s:%d", config.ServerAddress, config.ServerPort),
				KeepAlive: 25 * time.Second,
				Headers:   http.Header{},
				TLS: chclient.TLSConfig{
					SkipVerify: skipVerify,
				},
			}

			c, err_client := chclient.NewClient(clientConfig)
			if err_client != nil {
				err = err_client
			} else {
				err = c.Start(ctx)
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

			serv, err_server := chserver.NewServer(serverConfig)
			if err_server != nil {
				err = err_server
			} else {
				host := "0.0.0.0"
				port := fmt.Sprintf("%d", config.ListenPort)
				err = serv.StartContext(ctx, host, port)
			}
		}

		if err != nil && err != context.Canceled {
			log.Printf("Error running chisel service '%s': %v", config.Name, err)
		}
	}()

	config.PID = 1
	return s.db.Save(config).Error
}

func (s *ChiselService) StopChisel(config *model.ChiselConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	cancel, exists := s.activeServices[config.ID]
	if !exists {
		if config.PID != 0 {
			config.PID = 0
			s.db.Save(config)
		}
		return fmt.Errorf("service '%s' is not running", config.Name)
	}

	cancel()
	delete(s.activeServices, config.ID)
	return nil
}
