package app

import (
	"context"
	"encoding/json"
	"log"
	"os"

	"github.com/alireza0/s-ui/config"
	"github.com/alireza0/s-ui/core"
	"github.com/alireza0/s-ui/cronjob"
	"github.com/alireza0/s-ui/database"
	"github.com/alireza0/s-ui/database/model"
	"github.com/alireza0/s-ui/logger"
	"github.com/alireza0/s-ui/service"
	"github.com/alireza0/s-ui/sub"
	"github.com/alireza0/s-ui/telegram"
	"github.com/alireza0/s-ui/web"

	"github.com/op/go-logging"
)

type APP struct {
	service.SettingService
	configService  *service.ConfigService
	statsService   *service.StatsService
	serverService  *service.ServerService // New field for server service
	chiselService  *service.ChiselService
	mtprotoService *service.MTProtoService // Added
	greService     *service.GreService     // Added
	tapService     *service.TapService     // Added
	webServer      *web.Server
	subServer      *sub.Server
	cronJob        *cronjob.CronJob
	logger         *logging.Logger
	core           *core.Core
	telegramConfig *telegram.Config
	isBotStarted   bool
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
	a.mtprotoService = service.NewMTProtoService() // Added
	a.greService = service.NewGreService()         // Added
	a.tapService = service.NewTapService()         // Added
	a.configService = service.NewConfigService(a.core, a.chiselService)

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
			Name:          "defauilt",
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
	// --- End auto-start Chisel clients ---

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
		// If Init fails, we should probably exit, as the app is in an inconsistent state.
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

func (a *APP) GetMTProtoService() *service.MTProtoService {
	return a.mtprotoService
}

func (a *APP) GetGreService() *service.GreService {
	return a.greService
}

func (a *APP) GetTapService() *service.TapService {
	return a.tapService
}

