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

	"github.com/alireza0/s-ui/database"
	"github.com/alireza0/s-ui/database/model"
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
	return db.Delete(&model.ChiselConfig{}, id).Error
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


func (s *ChiselService) StartChisel(config *model.ChiselConfig) error {
	db := database.GetDB()
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
			goroutineDB := database.GetDB()
			if goroutineDB.First(&dbConfig, config.ID).Error == nil {
				if dbConfig.PID != 0 {
					dbConfig.PID = 0
					goroutineDB.Save(&dbConfig)
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

			serverURL := fmt.Sprintf("%s:%d", config.ServerAddress, config.ServerPort)
			if skipVerify { // A bit of a hack, but if we're skipping verify, we're probably using TLS
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
	return db.Save(config).Error
}

func (s *ChiselService) StopChisel(config *model.ChiselConfig) error {
	db := database.GetDB()
	s.mu.Lock()
	defer s.mu.Unlock()

	cancel, exists := s.activeServices[config.ID]
	if !exists {
		if config.PID != 0 {
			config.PID = 0
			db.Save(config)
		}
		return fmt.Errorf("service '%s' is not running", config.Name)
	}

	cancel()
	delete(s.activeServices, config.ID)
	return nil
}