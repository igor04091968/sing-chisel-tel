package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/alireza0/s-ui/database/model"
	"github.com/alireza0/s-ui/service"
	"github.com/alireza0/s-ui/util"
	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/gofrs/uuid/v5"
)

// AppServices defines the interface the bot needs to interact with the main app
type AppServices interface {
	RestartApp()
	GetConfigService() *service.ConfigService
	GetChiselService() *service.ChiselService
	GetFirstInboundId() (uint, error)
	GetUserByEmail(email string) (*model.Client, error)
	GetAllUsers() (*[]model.Client, error)
	FromIds(ids []uint) ([]*model.Inbound, error)
	GetOnlines() (*service.Onlines, error)
	GetLogs(limit string, level string) []string
	GetWebDomain() (string, error)
	GetInboundByTag(tag string) (*model.Inbound, error)
	BackupDB(exclude string) ([]byte, error)
	GetAllInbounds() ([]model.Inbound, error)
	GetAllOutbounds() ([]model.Outbound, error)
}

var (
	adminIDs   = make(map[int64]bool)
	services   AppServices
	currentBot *bot.Bot
)

// Start initializes and starts the Telegram bot
func Start(ctx context.Context, config *Config, appServices AppServices) {
	if !config.Enabled || config.BotToken == "" {
		log.Println("Telegram bot is disabled or token is not configured.")
		return
	}

	services = appServices

	for _, id := range config.AdminUserIDs {
		adminIDs[id] = true
	}

	opts := []bot.Option{
		bot.WithDefaultHandler(handler),
	}

	b, err := bot.New(config.BotToken, opts...)
	if err != nil {
		log.Printf("Error creating Telegram bot: %v", err)
		return
	}
	currentBot = b

	log.Println("Telegram bot started.")
	b.Start(ctx)
}

func Stop() {
	if currentBot != nil {
		currentBot.Close(context.Background())
		currentBot = nil
	}
}

func handler(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}

	userID := update.Message.From.ID
	if !isAdmin(userID) {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "You are not authorized to use this bot.",
		})
		return
	}

	if strings.HasPrefix(update.Message.Text, "/") {
		handleCommand(ctx, b, update.Message)
	}
}

func isAdmin(userID int64) bool {
	_, ok := adminIDs[userID]
	return ok
}

func handleCommand(ctx context.Context, b *bot.Bot, message *models.Message) {
	command, args := parseCommand(message.Text)

	switch command {
	case "/start":
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: message.Chat.ID,
			Text:   "Welcome to S-UI Admin Bot. Send /help to see available commands.",
		})
	case "/help":
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: message.Chat.ID,
			Text: "Available commands:\n" +
				"/adduser <email> <traffic_gb> [inbound_tag]\n" +
				"/deluser <email>\n" +
				"/stats\n" +
				"/logs\n" +
				"/restart\n" +
				"/sublink <email>\n" +
				"/list_users\n" +
				"/backup\n" +
				"/add_in <type> <tag> <port>\n" +
				"/add_out <json>\n" +
				"/list_inbounds\n" +
				"/list_outbounds\n\n" +
				"Chisel Commands:\n" +
				"/add_chisel_server <name> <port> [extra_args]\n" +
				"/add_chisel_client <name> <server:port> <remotes> [extra_args]\n" +
				"/list_chisel\n" +
				"/remove_chisel <name>\n" +
				"/start_chisel <name>\n" +
				"/stop_chisel <name>",
		})
	case "/adduser":
		handleAddUser(ctx, b, message, args)
	case "/deluser":
		handleDelUser(ctx, b, message, args)
	case "/stats":
		handleStats(ctx, b, message)
	case "/logs":
		handleLogs(ctx, b, message, args)
	case "/restart":
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: "Restarting s-ui service..."})
		services.RestartApp()
	case "/sublink":
		handleSublink(ctx, b, message, args)
	case "/add_in":
		handleAddInbound(ctx, b, message, args)
	case "/add_out":
		handleAddOutbound(ctx, b, message, args)
	case "/list_users":
		handleListUsers(ctx, b, message)
	case "/setup_service":
		handleSetupService(ctx, b, message)
	case "/backup":
		handleBackup(ctx, b, message)
	case "/add_chisel_server":
		handleAddChiselServer(ctx, b, message, args)
	case "/add_chisel_client":
		handleAddChiselClient(ctx, b, message, args)
	case "/list_chisel":
		handleListChisel(ctx, b, message)
	case "/remove_chisel":
		handleRemoveChisel(ctx, b, message, args)
	case "/start_chisel":
		handleStartChisel(ctx, b, message, args)
	case "/stop_chisel":
		handleStopChisel(ctx, b, message, args)
	case "/list_inbounds":
		handleListInbounds(ctx, b, message)
	case "/list_outbounds":
		handleListOutbounds(ctx, b, message)
	default:
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: message.Chat.ID,
			Text:   "Unknown command. Send /help to see available commands.",
		})
	}
}

func handleAddUser(ctx context.Context, b *bot.Bot, message *models.Message, args []string) {
	if len(args) < 2 || len(args) > 3 {
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: "Usage: /adduser <email> <traffic_gb> [inbound_tag]"})
		return
	}

	email := args[0]
	trafficGB, err := strconv.ParseInt(args[1], 10, 64)
	if err != nil {
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: "Invalid traffic value. It must be a number."})
		return
	}

	var inboundID uint
	if len(args) == 3 {
		inboundTag := args[2]
		inbound, err := services.GetInboundByTag(inboundTag)
		if err != nil {
			log.Printf("Error getting inbound by tag %s: %v", inboundTag, err)
			b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: fmt.Sprintf("Inbound with tag '%s' not found.", inboundTag)})
			return
		}
		inboundID = inbound.Id
	} else {
		inboundID, err = services.GetFirstInboundId()
		if err != nil {
			log.Printf("Error getting first inbound ID: %v", err)
			b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: "Error getting inbound ID."})
			return
		}
	}

	newUUID, err := uuid.NewV4()
	if err != nil {
		log.Printf("Error creating UUID: %v", err)
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: "Error creating user UUID."})
		return
	}

	clientConfig := fmt.Sprintf(`{"id": "%s"}`, newUUID.String())
	inboundsJSON := fmt.Sprintf(`[%d]`, inboundID)

	newClient := model.Client{
		Enable:   true,
		Name:     email,
		Volume:   trafficGB * 1024 * 1024 * 1024,
		Config:   json.RawMessage(clientConfig),
		Inbounds: json.RawMessage(inboundsJSON),
		Links:    json.RawMessage("[]"),
	}

	clientJSON, err := json.Marshal(newClient)
	if err != nil {
		log.Printf("Error marshalling client: %v", err)
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: "Error preparing user data."})
		return
	}

	_, err = services.GetConfigService().Save("clients", "new", clientJSON, "", "telegram-bot", "")
	if err != nil {
		log.Printf("Error saving client: %v", err)
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: fmt.Sprintf("Error adding user: %v", err)})
		return
	}

	b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: fmt.Sprintf("User %s added successfully.", email)})
}

func handleDelUser(ctx context.Context, b *bot.Bot, message *models.Message, args []string) {
	if len(args) != 1 {
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: "Usage: /deluser <email>"})
		return
	}

	email := args[0]
	client, err := services.GetUserByEmail(email)
	if err != nil {
		log.Printf("Error finding user %s: %v", email, err)
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: fmt.Sprintf("User %s not found.", email)})
		return
	}

	idJson, err := json.Marshal(client.Id)
	if err != nil {
		log.Printf("Error marshalling user ID: %v", err)
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: "Error preparing user ID for deletion."})
		return
	}

	_, err = services.GetConfigService().Save("clients", "del", idJson, "", "telegram-bot", "")
	if err != nil {
		log.Printf("Error deleting client: %v", err)
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: fmt.Sprintf("Error deleting user: %v", err)})
		return
	}

	b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: fmt.Sprintf("User %s deleted successfully.", email)})
}

func handleSublink(ctx context.Context, b *bot.Bot, message *models.Message, args []string) {
	if len(args) != 1 {
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: "Usage: /sublink <email>"})
		return
	}

	email := args[0]
	log.Printf("handleSublink: received request for email: %s", email)

	client, err := services.GetUserByEmail(email)
	if err != nil {
		log.Printf("Error finding user %s: %v", email, err)
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: fmt.Sprintf("User %s not found.", email)})
		return
	}
	log.Printf("handleSublink: found client: %+v", client)

	var inboundIDs []uint
	if err := json.Unmarshal(client.Inbounds, &inboundIDs); err != nil {
		log.Printf("Error unmarshalling inbounds for user %s: %v", email, err)
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: "Error reading user inbounds."})
		return
	}
	log.Printf("handleSublink: found inbound IDs: %v", inboundIDs)

	inbounds, err := services.FromIds(inboundIDs)
	if err != nil {
		log.Printf("Error getting inbounds for user %s: %v", email, err)
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: "Error getting user inbounds."})
		return
	}
	log.Printf("handleSublink: found inbounds: %+v", inbounds)

	webDomain, err := services.GetWebDomain()
	if err != nil {
		log.Printf("Error getting web domain: %v", err)
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: "Error getting web domain."})
		return
	}
	log.Printf("handleSublink: found web domain: %s", webDomain)

	var allLinks []string
	for _, inbound := range inbounds {
		links := util.LinkGenerator(client.Config, inbound, webDomain)
		allLinks = append(allLinks, links...)
	}
	log.Printf("handleSublink: generated links: %v", allLinks)

	if len(allLinks) == 0 {
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: fmt.Sprintf("No subscription links found for user %s.", email)})
		return
	}

	b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: "Subscription links for " + email + ":\n" + strings.Join(allLinks, "\n")})
}

func handleStats(ctx context.Context, b *bot.Bot, message *models.Message) {
	onlines, err := services.GetOnlines()
	if err != nil {
		log.Printf("Error getting online users: %v", err)
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: "Error getting online users."})
		return
	}

	var response strings.Builder
	response.WriteString("Online Users:\n")
	if len(onlines.User) > 0 {
		for _, user := range onlines.User {
			response.WriteString(fmt.Sprintf("- %s\n", user))
		}
	} else {
		response.WriteString("No users online.\n")
	}

	b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: response.String()})
}

func handleLogs(ctx context.Context, b *bot.Bot, message *models.Message, args []string) {
	limit := "10" // Default limit
	level := "debug" // Default level

	if len(args) > 0 {
		limit = args[0]
	}
	if len(args) > 1 {
		level = args[1]
	}

	logs := services.GetLogs(limit, level)
	if len(logs) == 0 {
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: "No logs found."})
		return
	}

	response := strings.Join(logs, "\n")
	b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: "Logs:\n" + response})
}

// handleAddOutbound still expects a full JSON configuration.
func handleAddOutbound(ctx context.Context, b *bot.Bot, message *models.Message, args []string) {
	if len(args) < 1 {
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: "Usage: /add_out <json_config>"})
		return
	}

	jsonData := []byte(strings.Join(args, " "))
	if !json.Valid(jsonData) {
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: "Invalid JSON provided."})
		return
	}

	_, err := services.GetConfigService().Save("outbounds", "new", jsonData, "", "telegram-bot", "")
	if err != nil {
		log.Printf("Error adding outbound: %v", err)
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: fmt.Sprintf("Error adding outbound: %v", err)})
		return
	}

	b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: "Outbound added successfully."})
}

func handleAddInbound(ctx context.Context, b *bot.Bot, message *models.Message, args []string) {
	if len(args) != 3 {
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: "Usage: /add_in <type> <tag> <port>"})
		return
	}

	inboundType := args[0]
	tag := args[1]
	port, err := strconv.Atoi(args[2])
	if err != nil {
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: "Invalid port number."})
		return
	}

	inboundConfig := map[string]interface{}{
		"type":        inboundType,
		"tag":         tag,
		"listen":      "0.0.0.0",
		"listen_port": port,
	}

	jsonData, err := json.Marshal(inboundConfig)
	if err != nil {
		log.Printf("Error marshalling inbound config: %v", err)
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: "Error creating inbound configuration."})
		return
	}

	_, err = services.GetConfigService().Save("inbounds", "new", jsonData, "", "telegram-bot", "")
	if err != nil {
		log.Printf("Error adding inbound: %v", err)
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: fmt.Sprintf("Error adding inbound: %v", err)})
		return
	}

	b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: fmt.Sprintf("Inbound of type '%s' with tag '%s' on port %d added successfully.", inboundType, tag, port)})
}

func handleBackup(ctx context.Context, b *bot.Bot, message *models.Message) {
	b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: "Creating backup..."})

	dbBytes, err := services.BackupDB("")
	if err != nil {
		log.Printf("Error creating backup: %v", err)
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: "Error creating backup."})
		return
	}

	fileName := fmt.Sprintf("s-ui-backup-%s.db", time.Now().Format("2006-01-02-15-04-05"))
	b.SendDocument(ctx, &bot.SendDocumentParams{
		ChatID:   message.Chat.ID,
		Document: &models.InputFileUpload{Filename: fileName, Data: bytes.NewReader(dbBytes)},
		Caption:  "Here is your database backup.",
	})
}

func handleSetupService(ctx context.Context, b *bot.Bot, message *models.Message) {
	serviceContent := "[Unit]\n" +
		"Description=s-ui Service\n" +
		"After=network.target\n" +
		"Wants=network.target\n\n" +
		"[Service]\n" +
		"Type=simple\n" +
		"WorkingDirectory=/source/s-ui/\n" +
		"ExecStart=/source/s-ui/sui\n" +
		"Restart=always\n" +
		"RestartSec=10s\n" +
		"LimitNOFILE=1048576\n\n" +
		"[Install]\n" +
		"WantedBy=multi-user.target"

	response := fmt.Sprintf("Для настройки сервиса systemd выполните следующие шаги:\n\n"+
		"**1. Создайте файл сервиса:**\n"+
		"Создайте файл `/etc/systemd/system/s-ui.service` со следующим содержимым (потребуются права sudo):\n"+
		"```ini\n%s\n```\n\n"+
		"**2. Выполните команды в терминале:**\n"+
		"```bash\n"+
		"sudo systemctl daemon-reload\n"+
		"sudo systemctl enable s-ui.service\n"+
		"sudo systemctl start s-ui.service\n"+
		"sudo systemctl status s-ui.service\n"+
		"```\n\n"+
		"После этого приложение будет запускаться как сервис в системе", serviceContent)

	b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: response})
}

func handleListUsers(ctx context.Context, b *bot.Bot, message *models.Message) {
	clients, err := services.GetAllUsers()
	if err != nil {
		log.Printf("Error getting all users: %v", err)
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: "Error getting users."})
		return
	}

	if len(*clients) == 0 {
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: "No users found."})
		return
	}

	var response strings.Builder
	response.WriteString("Users:\n")
	for _, client := range *clients {
		response.WriteString(fmt.Sprintf("- Name: %s, Enabled: %t\n", client.Name, client.Enable))
	}

	b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: response.String()})
}

func parseCommand(text string) (string, []string) {
	parts := strings.Fields(text)
	if len(parts) == 0 {
		return "", nil
	}
	return parts[0], parts[1:]
}

func handleAddChiselServer(ctx context.Context, b *bot.Bot, message *models.Message, args []string) {
	if len(args) < 2 {
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: "Usage: /add_chisel_server <name> <port> [extra_args]"})
		return
	}

	name := args[0]
	port, err := strconv.Atoi(args[1])
	if err != nil {
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: "Invalid port number."})
		return
	}

	extraArgs := ""
	if len(args) > 2 {
		extraArgs = strings.Join(args[2:], " ")
	}

	config := model.ChiselConfig{
		Name:          name,
		Mode:          "server",
		ListenAddress: "0.0.0.0",
		ListenPort:    port,
		Args:          extraArgs,
	}

	chiselService := services.GetChiselService()
	if err := chiselService.CreateChiselConfig(&config); err != nil {
		log.Printf("Error creating chisel config: %v", err)
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: fmt.Sprintf("Error creating config: %v", err)})
		return
	}

	b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: fmt.Sprintf("Chisel server config '%s' created. Starting...", name)})

	if err := chiselService.StartChisel(&config); err != nil {
		log.Printf("Error starting chisel server: %v", err)
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: fmt.Sprintf("Error starting server: %v", err)})
		return
	}

	b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: fmt.Sprintf("Chisel server '%s' started successfully on port %d.", name, port)})
}

func handleAddChiselClient(ctx context.Context, b *bot.Bot, message *models.Message, args []string) {
	if len(args) < 3 {
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: "Usage: /add_chisel_client <name> <server:port> <remotes> [extra_args]"})
		return
	}

	name := args[0]
	serverAddr := args[1]
	remotes := args[2]

	serverParts := strings.Split(serverAddr, ":")
	if len(serverParts) != 2 {
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: "Invalid server address format. Use <server:port>."})
		return
	}
	serverHost := serverParts[0]
	serverPort, err := strconv.Atoi(serverParts[1])
	if err != nil {
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: "Invalid server port."})
		return
	}

	allArgs := []string{remotes}
	if len(args) > 3 {
		allArgs = append(allArgs, args[3:]...)
	}
	extraArgs := strings.Join(allArgs, " ")

	config := model.ChiselConfig{
		Name:          name,
		Mode:          "client",
		ServerAddress: serverHost,
		ServerPort:    serverPort,
		Args:          extraArgs,
	}

	chiselService := services.GetChiselService()
	if err := chiselService.CreateChiselConfig(&config); err != nil {
		log.Printf("Error creating chisel config: %v", err)
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: fmt.Sprintf("Error creating config: %v", err)})
		return
	}

	b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: fmt.Sprintf("Chisel client config '%s' created. Starting...", name)})

	if err := chiselService.StartChisel(&config); err != nil {
		log.Printf("Error starting chisel client: %v", err)
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: fmt.Sprintf("Error starting client: %v", err)})
		return
	}

	b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: fmt.Sprintf("Chisel client '%s' started successfully.", name)})
}

func handleListChisel(ctx context.Context, b *bot.Bot, message *models.Message) {
	chiselService := services.GetChiselService()
	configs, err := chiselService.GetAllChiselConfigs()
	if err != nil {
		log.Printf("Error getting chisel configs: %v", err)
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: "Error getting chisel configs."})
		return
	}

	if len(configs) == 0 {
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: "No Chisel services configured."})
		return
	}

	var response strings.Builder
	response.WriteString("Configured Chisel Services:\n")
	for _, config := range configs {
		status := "Stopped"
		if config.PID > 0 {
			status = fmt.Sprintf("Running")
		}
		response.WriteString(fmt.Sprintf("\n- Name: *%s*\n", config.Name))
		response.WriteString(fmt.Sprintf("  Mode: %s\n", config.Mode))
		if config.Mode == "server" {
			response.WriteString(fmt.Sprintf("  Listen: 0.0.0.0:%d\n", config.ListenPort))
		} else {
			response.WriteString(fmt.Sprintf("  Server: %s:%d\n", config.ServerAddress, config.ServerPort))
			response.WriteString(fmt.Sprintf("  Remotes: `%s`\n", config.Args))
		}
		response.WriteString(fmt.Sprintf("  Status: %s\n", status))
		if config.Mode == "server" && config.Args != "" {
			response.WriteString(fmt.Sprintf("  Extra Args: `%s`\n", config.Args))
		}
	}

	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    message.Chat.ID,
		Text:      response.String(),
		ParseMode: models.ParseModeMarkdown,
	})
}

func handleRemoveChisel(ctx context.Context, b *bot.Bot, message *models.Message, args []string) {
	if len(args) != 1 {
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: "Usage: /remove_chisel <name>"})
		return
	}
	name := args[0]
	chiselService := services.GetChiselService()
	config, err := chiselService.GetChiselConfigByName(name)
	if err != nil {
		log.Printf("Error getting chisel config by name %s: %v", name, err)
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: fmt.Sprintf("Config with name '%s' not found.", name)})
		return
	}

	// Stop the service if it's running
	if config.PID > 0 {
		if err := chiselService.StopChisel(config); err != nil {
			log.Printf("Error stopping chisel service %s before removing: %v", name, err)
			b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: fmt.Sprintf("Service '%s' was running, attempted to stop it before removal. It will be removed anyway.", name)})
		} else {
			b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: fmt.Sprintf("Service '%s' stopped.", name)})
		}
	}

	if err := chiselService.DeleteChiselConfig(config.ID); err != nil {
		log.Printf("Error deleting chisel config %s: %v", name, err)
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: fmt.Sprintf("Error deleting config '%s': %v", name, err)})
		return
	}

	b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: fmt.Sprintf("Chisel config '%s' removed successfully.", name)})
}

func handleStartChisel(ctx context.Context, b *bot.Bot, message *models.Message, args []string) {
	if len(args) != 1 {
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: "Usage: /start_chisel <name>"})
		return
	}
	name := args[0]
	chiselService := services.GetChiselService()
	config, err := chiselService.GetChiselConfigByName(name)
	if err != nil {
		log.Printf("Error getting chisel config by name %s: %v", name, err)
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: fmt.Sprintf("Config with name '%s' not found.", name)})
		return
	}

	if config.PID > 0 {
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: fmt.Sprintf("Chisel service '%s' is already marked as running.", name)})
		return
	}

	if err := chiselService.StartChisel(config); err != nil {
		log.Printf("Error starting chisel service %s: %v", name, err)
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: fmt.Sprintf("Error starting chisel service '%s': %v", name, err)})
		return
	}

	b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: fmt.Sprintf("Chisel service '%s' started successfully.", name)})
}

func handleStopChisel(ctx context.Context, b *bot.Bot, message *models.Message, args []string) {
	if len(args) != 1 {
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: "Usage: /stop_chisel <name>"})
		return
	}
	name := args[0]
	chiselService := services.GetChiselService()
	config, err := chiselService.GetChiselConfigByName(name)
	if err != nil {
		log.Printf("Error getting chisel config by name %s: %v", name, err)
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: fmt.Sprintf("Config with name '%s' not found.", name)})
		return
	}

	if config.PID == 0 {
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: fmt.Sprintf("Chisel service '%s' is not marked as running.", name)})
		return
	}

	if err := chiselService.StopChisel(config); err != nil {
		log.Printf("Error stopping chisel service %s: %v", name, err)
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: fmt.Sprintf("Error stopping chisel service '%s': %v", name, err)})
		return
	}

	b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: fmt.Sprintf("Chisel service '%s' stopped successfully.", name)})
}

func handleListInbounds(ctx context.Context, b *bot.Bot, message *models.Message) {
	inbounds, err := services.GetAllInbounds()
	if err != nil {
		log.Printf("Error getting all inbounds: %v", err)
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: "Error getting inbounds."})
		return
	}

	if len(inbounds) == 0 {
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: "No inbounds configured."})
		return
	}

	var response strings.Builder
	response.WriteString("Configured Inbounds:\n")
	for _, inbound := range inbounds {
		response.WriteString(fmt.Sprintf("- ID: %d, Tag: %s, Type: %s\n", inbound.Id, inbound.Tag, inbound.Type))
	}

	b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: response.String()})
}

func handleListOutbounds(ctx context.Context, b *bot.Bot, message *models.Message) {
	outbounds, err := services.GetAllOutbounds()
	if err != nil {
		log.Printf("Error getting all outbounds: %v", err)
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: "Error getting outbounds."})
		return
	}

	if len(outbounds) == 0 {
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: "No outbounds configured."})
		return
	}

	var response strings.Builder
	response.WriteString("Configured Outbounds:\n")
	for _, outbound := range outbounds {
		response.WriteString(fmt.Sprintf("- ID: %d, Tag: %s, Type: %s\n", outbound.Id, outbound.Tag, outbound.Type))
	}

	b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: response.String()})
}

