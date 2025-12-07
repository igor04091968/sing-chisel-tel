package telegram

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/igor04091968/sing-chisel-tel/core"
	"github.com/igor04091968/sing-chisel-tel/database/model"
	"github.com/igor04091968/sing-chisel-tel/service"
	"gopkg.in/telebot.v3"
)

// AppServices is an interface that defines the methods the bot can use to interact with the main application.
type AppServices interface {
	GetCore() *core.Core
	GetConfigService() *service.ConfigService
	GetChiselService() *service.ChiselService
	GetGostService() *service.GostService
	GetUdpTunnelService() *service.UdpTunnelService
	GetMTProtoService() *service.MTProtoEmbeddedService
	GetGreService() *service.GreService
	GetTapService() *service.TapService
	GetFirstInboundId() (uint, error)
	GetUserByEmail(email string) (*model.Client, error)
	FromIds(ids []uint) ([]*model.Inbound, error)
	GetOnlines() (*service.Onlines, error)
	GetLogs(limit string, level string) []string
	GetInboundByTag(tag string) (*model.Inbound, error)
	BackupDB(exclude string) ([]byte, error)
	GetAllUsers() (*[]model.Client, error)
	GetAllInbounds() ([]model.Inbound, error)
	GetAllOutbounds() ([]model.Outbound, error)
	RestartApp()
}

// Start initializes and starts the Telegram bot.
func Start(ctx context.Context, cfg *Config, app AppServices) {
	if cfg == nil || !cfg.Enabled {
		return
	}

	pref := telebot.Settings{
		Token:  cfg.BotToken,
		Poller: &telebot.LongPoller{Timeout: 10},
	}

	bot, err := telebot.NewBot(pref)
	if err != nil {
		log.Fatalf("Failed to create Telegram bot: %v", err)
	}

	// Middleware to check for admin user
	adminOnly := bot.Group()
	adminOnly.Use(func(next telebot.HandlerFunc) telebot.HandlerFunc {
		return func(c telebot.Context) error {
			isAdmin := false
			for _, adminID := range cfg.AdminUserIDs {
				if c.Sender().ID == int64(adminID) {
					isAdmin = true
					break
				}
			}
			if !isAdmin {
				return c.Send("Access denied.")
			}
			return next(c)
		}
	})

	// Register handlers
	registerHandlers(adminOnly, app)

	log.Println("Telegram bot started...")
	bot.Start()
}

func registerHandlers(b *telebot.Group, app AppServices) {
	// UDP Tunnel Handlers
	b.Handle("/add_udptunnel", func(c telebot.Context) error {
		return handleAddUdpTunnel(c, app.GetUdpTunnelService())
	})
	b.Handle("/list_udptunnels", func(c telebot.Context) error {
		return handleListUdpTunnels(c, app.GetUdpTunnelService())
	})
	b.Handle("/remove_udptunnel", func(c telebot.Context) error {
		return handleRemoveUdpTunnel(c, app.GetUdpTunnelService())
	})
	b.Handle("/start_udptunnel", func(c telebot.Context) error {
		return handleStartUdpTunnel(c, app.GetUdpTunnelService())
	})
	b.Handle("/stop_udptunnel", func(c telebot.Context) error {
		return handleStopUdpTunnel(c, app.GetUdpTunnelService())
	})

	// Add other handlers here from the original file if they existed
}

func handleAddUdpTunnel(c telebot.Context, udpTunnelService *service.UdpTunnelService) error {
	args := c.Args()
	if len(args) < 4 {
		return c.Send("Usage: /add_udptunnel <name> <mode> <listen_port> <remote_addr:port>")
	}

	name := args[0]
	mode := args[1]
	listenPort, err := strconv.Atoi(args[2])
	if err != nil {
		return c.Send("Invalid listen port number.")
	}
	remoteAddr := args[3]

	config := model.UdpTunnelConfig{
		Name:          name,
		Mode:          mode,
		ListenPort:    listenPort,
		RemoteAddress: remoteAddr,
		Status:        "stopped",
	}

	if err := udpTunnelService.CreateUdpTunnel(&config); err != nil {
		log.Printf("Error creating UDP Tunnel config: %v", err)
		return c.Send(fmt.Sprintf("Error creating config: %v", err))
	}

	if err := udpTunnelService.StartUdpTunnel(&config); err != nil {
		log.Printf("Error starting UDP Tunnel: %v", err)
		return c.Send(fmt.Sprintf("Error starting tunnel: %v", err))
	}

	return c.Send(fmt.Sprintf("UDP Tunnel '%s' created and started successfully.", name))
}

func handleListUdpTunnels(c telebot.Context, udpTunnelService *service.UdpTunnelService) error {
	configs, err := udpTunnelService.GetAllUdpTunnels()
	if err != nil {
		log.Printf("Error getting UDP Tunnel configs: %v", err)
		return c.Send("Error getting UDP Tunnel configs.")
	}

	if len(configs) == 0 {
		return c.Send("No UDP Tunnel services configured.")
	}

	var response strings.Builder
	response.WriteString("Configured UDP Tunnel Services:\n")
	for _, config := range configs {
		response.WriteString(fmt.Sprintf("\n- Name: %s (ID: %d)\n", config.Name, config.ID))
		response.WriteString(fmt.Sprintf("  Mode: %s\n", config.Mode))
		response.WriteString(fmt.Sprintf("  Listen Port: %d\n", config.ListenPort))
		response.WriteString(fmt.Sprintf("  Remote Address: %s\n", config.RemoteAddress))
		response.WriteString(fmt.Sprintf("  Status: %s\n", config.Status))
		if config.ProcessID != 0 {
			response.WriteString(fmt.Sprintf("  PID: %d\n", config.ProcessID))
		}
	}

	return c.Send(response.String())
}

func handleRemoveUdpTunnel(c telebot.Context, udpTunnelService *service.UdpTunnelService) error {
	args := c.Args()
	if len(args) != 1 {
		return c.Send("Usage: /remove_udptunnel <name>")
	}
	name := args[0]
	config, err := udpTunnelService.GetUdpTunnelByName(name)
	if err != nil {
		return c.Send(fmt.Sprintf("Config with name '%s' not found.", name))
	}

	if err := udpTunnelService.DeleteUdpTunnel(config.ID); err != nil {
		return c.Send(fmt.Sprintf("Error deleting config '%s': %v", name, err))
	}

	return c.Send(fmt.Sprintf("UDP Tunnel config '%s' removed successfully.", name))
}

func handleStartUdpTunnel(c telebot.Context, udpTunnelService *service.UdpTunnelService) error {
	args := c.Args()
	if len(args) != 1 {
		return c.Send("Usage: /start_udptunnel <name>")
	}
	name := args[0]
	config, err := udpTunnelService.GetUdpTunnelByName(name)
	if err != nil {
		return c.Send(fmt.Sprintf("Config with name '%s' not found.", name))
	}

	if config.Status == "running" {
		return c.Send(fmt.Sprintf("UDP Tunnel '%s' is already running.", name))
	}

	if err := udpTunnelService.StartUdpTunnel(config); err != nil {
		return c.Send(fmt.Sprintf("Error starting UDP Tunnel service '%s': %v", name, err))
	}

	return c.Send(fmt.Sprintf("UDP Tunnel service '%s' started successfully.", name))
}

func handleStopUdpTunnel(c telebot.Context, udpTunnelService *service.UdpTunnelService) error {
	args := c.Args()
	if len(args) != 1 {
		return c.Send("Usage: /stop_udptunnel <name>")
	}
	name := args[0]
	config, err := udpTunnelService.GetUdpTunnelByName(name)
	if err != nil {
		return c.Send(fmt.Sprintf("Config with name '%s' not found.", name))
	}

	if config.Status != "running" {
		return c.Send(fmt.Sprintf("UDP Tunnel '%s' is not running.", name))
	}

	if err := udpTunnelService.StopUdpTunnel(config.ID); err != nil {
		return c.Send(fmt.Sprintf("Error stopping UDP Tunnel service '%s': %v", name, err))
	}

	return c.Send(fmt.Sprintf("UDP Tunnel service '%s' stopped successfully.", name))
}

