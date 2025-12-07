package app

import (
	"context"
	"encoding/json"
	"log"
	"os"

	"github.com/igor04091968/sing-chisel-tel/config"
	"github.com/igor04091968/sing-chisel-tel/core"
	"github.com/igor04091968/sing-chisel-tel/cronjob"
	"github.com/igor04091968/sing-chisel-tel/database"
	"github.com/igor04091968/sing-chisel-tel/database/model"
	"github.com/igor04091968/sing-chisel-tel/logger"
	"github.com/igor04091968/sing-chisel-tel/service"
	"github.com/igor04091968/sing-chisel-tel/sub"
	"github.com/igor04091968/sing-chisel-tel/telegram"
	"github.com/igor04091968/sing-chisel-tel/web"

	"github.com/op/go-logging"
)

type APP struct {
	service.SettingService
	configService    *service.ConfigService
	statsService     *service.StatsService
	serverService    *service.ServerService
	chiselService    *service.ChiselService
	gostService      *service.GostService
	mtprotoService   *service.MTProtoEmbeddedService
	greService       *service.GreService
	tapService       *service.TapService
	udpTunnelService *service.UdpTunnelService
	webServer        *web.Server
	subServer        *sub.Server
	cronJob          *cronjob.CronJob
	logger           *logging.Logger
	core             *core.Core
	telegramConfig   *telegram.Config
	isBotStarted     bool
}

func NewApp() *APP {
	return &APP{}
}

func (a *APP) Init() error {
	log.Printf("%v %v", config.GetName(), config.GetVersion())

	a.initLog()

	err := database.InitDB(config.GetDBPath())
	if err != nil {
		return err
	}

	a.initTelegramConfig()

	// Init Setting
	a.SettingService.GetAllSetting()

	a.core = core.NewCore()

	a.cronJob = cronjob.NewCronJob()
	a.webServer = web.NewServer()
	a.subServer = sub.NewServer()

	a.chiselService = service.NewChiselService()
	a.gostService = service.NewGostService()
	a.mtprotoService = service.NewMTProtoEmbeddedService()
	a.greService = service.NewGreService()
	a.tapService = service.NewTapService()
	a.udpTunnelService = service.NewUdpTunnelService(database.GetDB())
	a.configService = service.NewConfigService(a.core, a.chiselService)

	// Initialize lightweight services that don't have complex constructors
	a.statsService = &service.StatsService{}
	a.serverService = &service.ServerService{}

	// Provide initialized services to the web server so API handlers can use them
	if a.webServer != nil {
		bundle := &service.ServicesBundle{
			SettingService:   a.SettingService,
			ConfigService:    a.configService,
			ClientService:    service.ClientService{},
			TlsService:       service.TlsService{},
			InboundService:   service.InboundService{},
			OutboundService:  service.OutboundService{},
			EndpointService:  service.EndpointService{},
			ServicesService:  service.ServicesService{},
			PanelService:     service.PanelService{},
			StatsService:     *a.statsService,
			ServerService:    *a.serverService,
			ChiselService:    a.chiselService,
			GostService:      a.gostService,
			TapService:       a.tapService,
			UdpTunnelService: a.udpTunnelService,
		}
		a.webServer.SetServicesBundle(bundle)
	}

	// --- Add default Chisel client config if none exists ---
	chiselClients, err := a.chiselService.GetAllChiselConfigs()
	if err != nil {
		logger.Error("Error checking for existing Chisel configs:", err)
		return err
	}

	hasClientConfig := false
	for _, cfg := range chiselClients {
		if cfg.Mode == "client" {
			hasClientConfig = true
			break
		}
	}

	if !hasClientConfig {
		defaultChiselConfig := model.ChiselConfig{
			Name:          "default",
			Mode:          "client",
			ServerAddress: "127.0.0.1",
			ServerPort:    8443,
			Args:          "--tls-skip-verify R:8000:localhost:8080",
		}
		if err := a.chiselService.CreateChiselConfig(&defaultChiselConfig); err != nil {
			logger.Error("Error creating default Chisel client config:", err)
			return err
		}
		logger.Info("Default Chisel client config 'default-chisel-client' created.")
	}
	// --- End default Chisel client config ---

	return nil
}

func (a *APP) Start() error {
	loc, err := a.SettingService.GetTimeLocation()
	if err != nil {
		return err
	}

	trafficAge, err := a.SettingService.GetTrafficAge()
	if err != nil {
		return err
	}

	err = a.cronJob.Start(loc, trafficAge)
	if err != nil {
		return err
	}

	err = a.webServer.Start()
	if err != nil {
		return err
	}

	err = a.subServer.Start()
	if err != nil {
		return err
	}

	err = a.configService.StartCore("")
	if err != nil {
		logger.Error(err)
	}

	if a.telegramConfig != nil && a.telegramConfig.Enabled && !a.isBotStarted {
		go telegram.Start(context.Background(), a.telegramConfig, a)
		a.isBotStarted = true
	}

	// --- Auto-start all Chisel clients ---
	allChiselConfigs, err := a.chiselService.GetAllChiselConfigs()
	if err != nil {
		logger.Error("Error getting all Chisel configs for PID reset:", err)
		return err
	}
	for _, cfg := range allChiselConfigs {
		if cfg.PID != 0 {
			cfg.PID = 0
			if err := a.chiselService.UpdateChiselConfig(&cfg); err != nil {
				logger.Warningf("Error resetting PID for Chisel config '%s' (ID: %d): %v", cfg.Name, cfg.ID, err)
			} else {
				logger.Infof("Reset PID for Chisel config '%s' (ID: %d) to 0.", cfg.Name, cfg.ID)
			}
		}
	}

	chiselConfigs, err := a.chiselService.GetAllChiselConfigs()
	if err != nil {
		logger.Error("Error getting all Chisel configs for auto-start:", err)
		return err
	}

	for _, cfg := range chiselConfigs {
		if cfg.Mode == "client" && cfg.PID == 0 && cfg.ServerAddress != "" && cfg.ServerPort != 0 {
			if err := a.chiselService.StartChisel(&cfg); err != nil {
				logger.Error("Error auto-starting Chisel client '", cfg.Name, "':", err)
			} else {
				logger.Info("Chisel client '", cfg.Name, "' auto-started.")
			}
		}
	}
	// --- Auto-start UDP Tunnels ---
	a.udpTunnelService.AutoStartUdpTunnels()
	// --- End auto-start UDP Tunnels ---

	return nil
}

func (a *APP) Stop() {
	a.cronJob.Stop()
	err := a.subServer.Stop()
	if err != nil {
		logger.Warning("stop Sub Server err:", err)
	}
	err = a.webServer.Stop()
	if err != nil {
		logger.Warning("stop Web Server err:", err)
	}
	err = a.configService.StopCore()
	if err != nil {
		logger.Warning("stop Core err:", err)
	}

	a.chiselService.StopAllActiveChiselServices()
}

func (a *APP) initLog() {
	switch config.GetLogLevel() {
	case config.Debug:
		logger.InitLogger(logging.DEBUG)
	case config.Info:
		logger.InitLogger(logging.INFO)
	case config.Warn:
		logger.InitLogger(logging.WARNING)
	case config.Error:
		logger.InitLogger(logging.ERROR)
	default:
		log.Fatal("unknown log level:", config.GetLogLevel())
	}
}

func (a *APP) initTelegramConfig() {
	file, err := os.ReadFile("telegram_config.json")
	if err != nil {
		if os.IsNotExist(err) {
			logger.Info("telegram_config.json not found, Telegram bot is disabled.")
			return
		}
		logger.Warning("Error reading telegram_config.json:", err)
		return
	}

	var cfg telegram.Config
	if err := json.Unmarshal(file, &cfg); err != nil {
		logger.Warning("Error unmarshalling telegram_config.json:", err)
		return
	}
	a.telegramConfig = &cfg
}

func (a *APP) RestartApp() {
	a.Stop()
	err := a.Init()
	if err != nil {
		logger.Error("Error re-initializing app:", err)
		os.Exit(1)
	}
	err = a.Start()
	if err != nil {
		logger.Error("Error re-starting app:", err)
		os.Exit(1)
	}
}

func (a *APP) GetCore() *core.Core {
	return a.core
}

func (a *APP) GetConfigService() *service.ConfigService {
	return a.configService
}

func (a *APP) GetChiselService() *service.ChiselService {
	return a.chiselService
}

func (a *APP) GetFirstInboundId() (uint, error) {
	return a.configService.GetFirstInboundId()
}

func (a *APP) GetUserByEmail(email string) (*model.Client, error) {
	return a.configService.GetUserByEmail(email)
}

func (a *APP) FromIds(ids []uint) ([]*model.Inbound, error) {
	return a.configService.FromIds(ids)
}

func (a *APP) GetOnlines() (*service.Onlines, error) {
	return a.statsService.GetOnlines()
}

func (a *APP) GetLogs(limit string, level string) []string {
	return a.serverService.GetLogs(limit, level)
}

func (a *APP) GetInboundByTag(tag string) (*model.Inbound, error) {
	return a.configService.GetInboundByTag(tag)
}

func (a *APP) BackupDB(exclude string) ([]byte, error) {
	return database.GetDb(exclude)
}

func (a *APP) GetAllUsers() (*[]model.Client, error) {
	return a.configService.GetAllUsers()
}

func (a *APP) GetAllInbounds() ([]model.Inbound, error) {
	return a.configService.GetAllInbounds()
}

func (a *APP) GetAllOutbounds() ([]model.Outbound, error) {
	return a.configService.GetAllOutbounds()
}

func (a *APP) GetMTProtoService() *service.MTProtoEmbeddedService {
	return a.mtprotoService
}

func (a *APP) GetGreService() *service.GreService {
	return a.greService
}

func (a *APP) GetTapService() *service.TapService {
	return a.tapService
}

func (a *APP) GetGostService() *service.GostService {
	return a.gostService
}

func (a *APP) GetUdpTunnelService() *service.UdpTunnelService {
	return a.udpTunnelService
}