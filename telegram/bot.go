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
	GetMTProtoService() *service.MTProtoService
	GetGreService() *service.GreService
	GetTapService() *service.TapService
	GetUdp2rawService() *service.Udp2rawService // Added
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
				"/add_chisel_client <name> <server:port> [R:local:remote ...] [extra_args]\n" +
				"/list_chisel\n" +
				"/remove_chisel <name>\n" +
				"/start_chisel <name>\n" +
				"/stop_chisel <name>\n\n" +
				"Subscription Domain Commands:\n" +
				"/set_sub_domain <domain>\n" +
				"/get_sub_domain\n\n" +
				"MTProto Proxy Commands:\n" +
				"/add_mtproto <name> <port> <secret> [ad_tag]\n" +
				"/list_mtproto\n" +
				"/remove_mtproto <name>\n" +
				"/start_mtproto <name>\n" +
				"/stop_mtproto <name>\n" +
				"/gen_mtproto_secret\n\n" +
				"GRE Tunnel Commands:\n" +
				"/add_gre <name> <local_ip> <remote_ip> [interface_name]\n" +
				"/list_gre\n" +
				"/remove_gre <name>\n" +
				"/start_gre <name>\n" +
				"/stop_gre <name>\n\n" +
				"TAP Tunnel Commands:\n" +
				"/add_tap <name> <ip_address> [mtu] [interface_name]\n" +
				"/list_tap\n" +
				"/remove_tap <name>\n" +
				"/start_tap <name>\n" +
				"/stop_tap <name>\n\n" +
				"goudp2raw Tunnel Commands:\n" +
				"/add_udp2raw_client <name> <local_addr> <remote_server_ip> <key> [dscp]\n" +
				"/add_udp2raw_server <name> <local_addr> <target_udp_service> <key> [dscp]\n" +
				"/list_udp2raw\n" +
				"/remove_udp2raw <name>\n" +
				"/start_udp2raw <name>\n" +
				"/stop_udp2raw <name>",
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
	// Subscription Domain Commands
	case "/set_sub_domain":
		handleSetSubDomain(ctx, b, message, args)
	case "/get_sub_domain":
		handleGetSubDomain(ctx, b, message)
	// MTProto Proxy Commands
	case "/add_mtproto":
		handleAddMTProto(ctx, b, message, args)
	case "/list_mtproto":
		handleListMTProto(ctx, b, message)
	case "/remove_mtproto":
		handleRemoveMTProto(ctx, b, message, args)
	case "/start_mtproto":
		handleStartMTProto(ctx, b, message, args)
	case "/stop_mtproto":
		handleStopMTProto(ctx, b, message, args)
	case "/gen_mtproto_secret":
		handleGenerateMTProtoSecret(ctx, b, message)
	// GRE Tunnel Commands
	case "/add_gre":
		handleAddGre(ctx, b, message, args)
	case "/list_gre":
		handleListGre(ctx, b, message)
	case "/remove_gre":
		handleRemoveGre(ctx, b, message, args)
	case "/start_gre":
		handleStartGre(ctx, b, message, args)
	case "/stop_gre":
		handleStopGre(ctx, b, message, args)
	// TAP Tunnel Commands
	case "/add_tap":
		handleAddTap(ctx, b, message, args)
	case "/list_tap":
		handleListTap(ctx, b, message)
	case "/remove_tap":
		handleRemoveTap(ctx, b, message, args)
	case "/start_tap":
		handleStartTap(ctx, b, message, args)
	case "/stop_tap":
		handleStopTap(ctx, b, message, args)
	// goudp2raw Tunnel Commands
	case "/add_udp2raw_client":
		handleAddUdp2rawClient(ctx, b, message, args)
	case "/add_udp2raw_server":
		handleAddUdp2rawServer(ctx, b, message, args)
	case "/list_udp2raw":
		handleListUdp2raw(ctx, b, message)
	case "/remove_udp2raw":
		handleRemoveUdp2raw(ctx, b, message, args)
	case "/start_udp2raw":
		handleStartUdp2raw(ctx, b, message, args)
	case "/stop_udp2raw":
		handleStopUdp2raw(ctx, b, message, args)
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
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: fmt.Sprintf("Error: User %s not found or database error: %v", email, err)})
		return
	}
	log.Printf("handleSublink: found client: %+v", client)

	var inboundIDs []uint
	if err := json.Unmarshal(client.Inbounds, &inboundIDs); err != nil {
		log.Printf("Error unmarshalling inbounds for user %s: %v", email, err)
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: fmt.Sprintf("Error: Could not read user inbounds configuration: %v", err)})
		return
	}
	if len(inboundIDs) == 0 {
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: fmt.Sprintf("No inbounds configured for user %s. Please add inbounds to the user.", email)})
		return
	}
	log.Printf("handleSublink: found inbound IDs: %v", inboundIDs)

	inbounds, err := services.FromIds(inboundIDs)
	if err != nil {
		log.Printf("Error getting inbounds for user %s: %v", email, err)
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: fmt.Sprintf("Error: Could not retrieve inbound details for user %s: %v", email, err)})
		return
	}
	if len(inbounds) == 0 {
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: fmt.Sprintf("No active inbounds found for user %s based on configured IDs.", email)})
		return
	}
	log.Printf("handleSublink: found inbounds: %+v", inbounds)

	webDomain, err := services.GetWebDomain()
	if err != nil {
		log.Printf("Error getting web domain: %v", err)
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: fmt.Sprintf("Error: Could not retrieve web domain for link generation: %v", err)})
		return
	}
	if webDomain == "" {
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: "Error: Web domain is not configured. Cannot generate subscription links."}) 
		return
	}
	log.Printf("handleSublink: found web domain: %s", webDomain)

	var allLinks []string
	for _, inbound := range inbounds {
		links := util.LinkGenerator(client.Config, inbound, webDomain)
		if len(links) > 0 {
			allLinks = append(allLinks, links...)
		} else {
			log.Printf("handleSublink: LinkGenerator returned no links for inbound ID %d, tag %s", inbound.Id, inbound.Tag)
		}
	}
	log.Printf("handleSublink: generated links: %v", allLinks)

	if len(allLinks) == 0 {
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: fmt.Sprintf("No subscription links could be generated for user %s. Check user configuration, inbounds, and web domain settings.", email)})
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

	response := fmt.Sprintf("Для настройки сервиса systemd выполните следующие шаги:\n\n" +
		"**1. Создайте файл сервиса:**\n" +
		"Создайте файл `/etc/systemd/system/s-ui.service` со следующим содержимым (потребуются права sudo):\n" +
		"```ini\n%s\n```\n\n" +
		"**2. Выполните команды в терминале:**\n" +
		"```bash\n" +
		"sudo systemctl daemon-reload\n" +
		"sudo systemctl enable s-ui.service\n" +
		"sudo systemctl start s-ui.service\n" +
		"sudo systemctl status s-ui.service\n" +
		"```\n\n" +
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
	// Usage: /add_chisel_client <name> <server:port> [remotes_and_extra_args...]
	if len(args) < 2 {
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: "Usage: /add_chisel_client <name> <server:port> [remotes_and_extra_args]"})
		return
	}

	name := args[0]
	serverAddr := args[1]

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

	var finalArgs []string

	// Always include default auth and TLS skip verify for HTTPS connection
	finalArgs = append(finalArgs, "--auth", "chisel:2025")
	finalArgs = append(finalArgs, "--tls-skip-verify")

	// Default remotes
	defaultRemotes := []string{
		"R:2095:localhost:2095",
		"R:2096:localhost:2096",
		"R:1025:localhost:1025",
		"R:1026:localhost:1026",
		"R:1027:localhost:1027",
		"R:1028:localhost:1028",
	}

	// Check if user provided any remotes or extra args
	if len(args) > 2 {
		userProvidedArgs := args[2:]
		hasExplicitRemotes := false
		for _, arg := range userProvidedArgs {
			if strings.HasPrefix(arg, "R:") {
				hasExplicitRemotes = true
				break
			}
		}

		if hasExplicitRemotes {
			// User provided explicit remotes, so append all their args directly
			finalArgs = append(finalArgs, userProvidedArgs...)
		} else {
			// No explicit remotes from user, so use defaults and append their other extra args
			finalArgs = append(finalArgs, defaultRemotes...)
			finalArgs = append(finalArgs, userProvidedArgs...)
		}
	} else {
		// No arguments after <server:port>, so use default remotes
		finalArgs = append(finalArgs, defaultRemotes...)
	}

	config := model.ChiselConfig{
		Name:          name,
		Mode:          "client",
		ServerAddress: serverHost,
		ServerPort:    serverPort,
		Args:          strings.Join(finalArgs, " "),
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
			status = "Running"
		}
		response.WriteString(fmt.Sprintf("\n- Name: %s\n", config.Name))
		response.WriteString(fmt.Sprintf("  Mode: %s\n", config.Mode))
		if config.Mode == "server" {
			response.WriteString(fmt.Sprintf("  Listen: 0.0.0.0:%d\n", config.ListenPort))
		} else {
			response.WriteString(fmt.Sprintf("  Server: %s:%d\n", config.ServerAddress, config.ServerPort))
			response.WriteString(fmt.Sprintf("  Args: %s\n", config.Args))
		}
		response.WriteString(fmt.Sprintf("  Status: %s\n", status))
		if config.Mode == "server" && config.Args != "" {
			response.WriteString(fmt.Sprintf("  Extra Args: %s\n", config.Args))
		}
	}

	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: message.Chat.ID,
		Text:   response.String(),
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

	// Attempt to stop the service first.
	if err := chiselService.StopChisel(config); err != nil {
		// This would likely be a DB error. Log it, but proceed with deletion attempt.
		log.Printf("Error stopping chisel service %s during removal (will attempt deletion anyway): %v", name, err)
	}

	// Now, delete the config from the database.
	if err := chiselService.DeleteChiselConfig(config.ID); err != nil {
		log.Printf("Error deleting chisel config %s: %v", name, err)
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: fmt.Sprintf("Failed to delete config '%s': %v", name, err)})
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

// MTProto Proxy Handlers
func handleAddMTProto(ctx context.Context, b *bot.Bot, message *models.Message, args []string) {
	if len(args) < 3 || len(args) > 4 {
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: "Usage: /add_mtproto <name> <port> <secret> [ad_tag]"})
		return
	}

	name := args[0]
	port, err := strconv.Atoi(args[1])
	if err != nil {
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: "Invalid port number."}) 
		return
	}
	secret := args[2]
	adTag := ""
	if len(args) == 4 {
		adTag = args[3]
	}

	config := model.MTProtoProxyConfig{
		Name:       name,
		ListenPort: port,
		Secret:     secret,
		AdTag:      adTag,
		Status:     "down", // Initially down
	}

	mtprotoService := services.GetMTProtoService()
	if err := mtprotoService.CreateMTProtoProxy(&config); err != nil {
		log.Printf("Error creating MTProto Proxy config: %v", err)
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: fmt.Sprintf("Error creating config: %v", err)})
		return
	}

	b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: fmt.Sprintf("MTProto Proxy config '%s' created. Starting...", name)})

	if err := mtprotoService.StartMTProtoProxy(&config); err != nil {
		log.Printf("Error starting MTProto Proxy: %v", err)
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: fmt.Sprintf("Error starting proxy: %v", err)})
		return
	}

	b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: fmt.Sprintf("MTProto Proxy '%s' started successfully on port %d.", name, port)})
}

func handleListMTProto(ctx context.Context, b *bot.Bot, message *models.Message) {
	mtprotoService := services.GetMTProtoService()
	configs, err := mtprotoService.GetAllMTProtoProxies()
	if err != nil {
		log.Printf("Error getting MTProto Proxy configs: %v", err)
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: "Error getting MTProto Proxy configs."}) 
		return
	}

	if len(configs) == 0 {
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: "No MTProto Proxy services configured."}) 
		return
	}

	var response strings.Builder
	response.WriteString("Configured MTProto Proxy Services:\n")
	for _, config := range configs {
		response.WriteString(fmt.Sprintf("\n- Name: %s\n", config.Name))
		response.WriteString(fmt.Sprintf("  Listen Port: %d\n", config.ListenPort))
		response.WriteString(fmt.Sprintf("  Secret: %s\n", config.Secret))
		if config.AdTag != "" {
			response.WriteString(fmt.Sprintf("  Ad Tag: %s\n", config.AdTag))
		}
		response.WriteString(fmt.Sprintf("  Status: %s\n", config.Status))
	}

	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: message.Chat.ID,
		Text:   response.String(),
	})
}

func handleRemoveMTProto(ctx context.Context, b *bot.Bot, message *models.Message, args []string) {
	if len(args) != 1 {
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: "Usage: /remove_mtproto <name>"})
		return
	}
	name := args[0]
	mtprotoService := services.GetMTProtoService()
	config, err := mtprotoService.GetMTProtoProxyByName(name) // Assuming GetMTProtoProxyByName exists
	if err != nil {
		log.Printf("Error getting MTProto Proxy config by name %s: %v", name, err)
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: fmt.Sprintf("Config with name '%s' not found.", name)})
		return
	}

	if config.Status == "up" {
		if err := mtprotoService.StopMTProtoProxy(config.ID); err != nil {
			log.Printf("Error stopping MTProto Proxy %s before removing: %v", name, err)
			b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: fmt.Sprintf("Proxy '%s' was running, attempted to stop it before removal. It will be removed anyway.", name)})
		} else {
			b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: fmt.Sprintf("Proxy '%s' stopped.", name)})
		}
	}

	if err := mtprotoService.DeleteMTProtoProxy(config.ID); err != nil {
		log.Printf("Error deleting MTProto Proxy config %s: %v", name, err)
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: fmt.Sprintf("Error deleting config '%s': %v", name, err)})
		return
	}

	b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: fmt.Sprintf("MTProto Proxy config '%s' removed successfully.", name)})
}

func handleStartMTProto(ctx context.Context, b *bot.Bot, message *models.Message, args []string) {
	if len(args) != 1 {
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: "Usage: /start_mtproto <name>"})
		return
	}
	name := args[0]
	mtprotoService := services.GetMTProtoService()
	config, err := mtprotoService.GetMTProtoProxyByName(name) // Assuming GetMTProtoProxyByName exists
	if err != nil {
		log.Printf("Error getting MTProto Proxy config by name %s: %v", name, err)
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: fmt.Sprintf("Config with name '%s' not found.", name)})
		return
	}

	if config.Status == "up" {
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: fmt.Sprintf("MTProto Proxy '%s' is already running.", name)})
		return
	}

	if err := mtprotoService.StartMTProtoProxy(config); err != nil {
		log.Printf("Error starting MTProto Proxy %s: %v", name, err)
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: fmt.Sprintf("Error starting proxy '%s': %v", name, err)})
		return
	}

	b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: fmt.Sprintf("MTProto Proxy '%s' started successfully.", name)})
}

func handleStopMTProto(ctx context.Context, b *bot.Bot, message *models.Message, args []string) {
	if len(args) != 1 {
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: "Usage: /stop_mtproto <name>"})
		return
	}
	name := args[0]
	mtprotoService := services.GetMTProtoService()
	config, err := mtprotoService.GetMTProtoProxyByName(name) // Assuming GetMTProtoProxyByName exists
	if err != nil {
		log.Printf("Error getting MTProto Proxy config by name %s: %v", name, err)
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: fmt.Sprintf("Config with name '%s' not found.", name)})
		return
	}

	if config.Status == "down" {
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: fmt.Sprintf("MTProto Proxy '%s' is not running.", name)})
		return
	}

	if err := mtprotoService.StopMTProtoProxy(config.ID); err != nil {
		log.Printf("Error stopping MTProto Proxy %s: %v", name, err)
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: fmt.Sprintf("Error stopping proxy '%s': %v", name, err)})
		return
	}

	b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: fmt.Sprintf("MTProto Proxy '%s' stopped successfully.", name)})
}

func handleGenerateMTProtoSecret(ctx context.Context, b *bot.Bot, message *models.Message) {
	secret, err := service.GenerateMTProtoSecret()
	if err != nil {
		log.Printf("Error generating MTProto secret: %v", err)
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: fmt.Sprintf("Error generating secret: %v", err)})
		return
	}
	b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: fmt.Sprintf("Generated MTProto Secret: `%s`", secret)})
}

// GRE Tunnel Handlers
func handleAddGre(ctx context.Context, b *bot.Bot, message *models.Message, args []string) {
	if len(args) < 3 || len(args) > 4 {
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: "Usage: /add_gre <name> <local_ip> <remote_ip> [interface_name]"})
		return
	}

	name := args[0]
	localIP := args[1]
	remoteIP := args[2]
	interfaceName := ""
	if len(args) == 4 {
		interfaceName = args[3]
	}

	config := model.GreTunnel{
		Name:          name,
		LocalAddress:  localIP,
		RemoteAddress: remoteIP,
		InterfaceName: interfaceName,
		Status:        "down", // Initially down
	}

	greService := services.GetGreService()
	if err := greService.CreateGreTunnel(&config); err != nil {
		log.Printf("Error creating GRE Tunnel config: %v", err)
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: fmt.Sprintf("Error creating config: %v", err)})
		return
	}

	b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: fmt.Sprintf("GRE Tunnel config '%s' created and started successfully.", name)})
}

func handleListGre(ctx context.Context, b *bot.Bot, message *models.Message) {
	greService := services.GetGreService()
	configs, err := greService.GetAllGreTunnels()
	if err != nil {
		log.Printf("Error getting GRE Tunnel configs: %v", err)
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: "Error getting GRE Tunnel configs."}) 
		return
	}

	if len(configs) == 0 {
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: "No GRE Tunnel services configured."}) 
		return
	}

	var response strings.Builder
	response.WriteString("Configured GRE Tunnel Services:\n")
	for _, config := range configs {
		response.WriteString(fmt.Sprintf("\n- Name: %s\n", config.Name))
		response.WriteString(fmt.Sprintf("  Local IP: %s\n", config.LocalAddress))
		response.WriteString(fmt.Sprintf("  Remote IP: %s\n", config.RemoteAddress))
		response.WriteString(fmt.Sprintf("  Interface Name: %s\n", config.InterfaceName))
		response.WriteString(fmt.Sprintf("  Status: %s\n", config.Status))
	}

	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: message.Chat.ID,
		Text:   response.String(),
	})
}

func handleRemoveGre(ctx context.Context, b *bot.Bot, message *models.Message, args []string) {
	if len(args) != 1 {
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: "Usage: /remove_gre <name>"})
		return
	}
	name := args[0]
	greService := services.GetGreService()
	config, err := greService.GetGreTunnelByName(name)
	if err != nil {
		log.Printf("Error getting GRE Tunnel config by name %s: %v", name, err)
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: fmt.Sprintf("Config with name '%s' not found.", name)})
		return
	}

	if err := greService.DeleteGreTunnel(config.ID); err != nil {
		log.Printf("Error deleting GRE Tunnel config %s: %v", name, err)
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: fmt.Sprintf("Error deleting config '%s': %v", name, err)})
		return
	}

	b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: fmt.Sprintf("GRE Tunnel config '%s' removed successfully.", name)})
}

func handleStartGre(ctx context.Context, b *bot.Bot, message *models.Message, args []string) {
	if len(args) != 1 {
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: "Usage: /start_gre <name>"})
		return
	}
	name := args[0]
	greService := services.GetGreService()
	config, err := greService.GetGreTunnelByName(name)
	if err != nil {
		log.Printf("Error getting GRE Tunnel config by name %s: %v", name, err)
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: fmt.Sprintf("Config with name '%s' not found.", name)})
		return
	}

	if config.Status == "up" {
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: fmt.Sprintf("GRE Tunnel '%s' is already running.", name)})
		return
	}

	// Re-create the tunnel
	if err := greService.CreateGreTunnel(config); err != nil {
		log.Printf("Error starting GRE Tunnel %s: %v", name, err)
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: fmt.Sprintf("Error starting tunnel '%s': %v", name, err)})
		return
	}

	b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: fmt.Sprintf("GRE Tunnel '%s' started successfully.", name)})
}

func handleStopGre(ctx context.Context, b *bot.Bot, message *models.Message, args []string) {
	if len(args) != 1 {
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: "Usage: /stop_gre <name>"})
		return
	}
	name := args[0]
	greService := services.GetGreService()
	config, err := greService.GetGreTunnelByName(name)
	if err != nil {
		log.Printf("Error getting GRE Tunnel config by name %s: %v", name, err)
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: fmt.Sprintf("Config with name '%s' not found.", name)})
		return
	}

	if config.Status == "down" {
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: fmt.Sprintf("GRE Tunnel '%s' is not running.", name)})
		return
	}

	// Delete the tunnel to stop it
	if err := greService.DeleteGreTunnel(config.ID); err != nil {
		log.Printf("Error stopping GRE Tunnel %s: %v", name, err)
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: fmt.Sprintf("Error stopping tunnel '%s': %v", name, err)})
		return
	}

	b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: fmt.Sprintf("GRE Tunnel '%s' stopped successfully.", name)})
}

// TAP Tunnel Handlers
func handleAddTap(ctx context.Context, b *bot.Bot, message *models.Message, args []string) {
	if len(args) < 2 || len(args) > 4 {
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: "Usage: /add_tap <name> <ip_address> [mtu] [interface_name]"})
		return
	}

	name := args[0]
	ipAddress := args[1]
	mtu := 0 // Default MTU
	interfaceName := ""

	if len(args) >= 3 {
		if val, err := strconv.Atoi(args[2]); err == nil {
			mtu = val
		} else {
			// If it's not an int, assume it's interface_name
			interfaceName = args[2]
		}
	}
	if len(args) == 4 {
		interfaceName = args[3]
	}

	config := model.TapTunnel{
		Name:          name,
		LocalAddress:  ipAddress,
		MTU:           mtu,
		InterfaceName: interfaceName,
		Status:        "down", // Initially down
	}

	tapService := services.GetTapService()
	if err := tapService.CreateTapTunnel(&config); err != nil {
		log.Printf("Error creating TAP Tunnel config: %v", err)
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: fmt.Sprintf("Error creating config: %v", err)})
		return
	}

	b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: fmt.Sprintf("TAP Tunnel config '%s' created and started successfully.", name)})
}

func handleListTap(ctx context.Context, b *bot.Bot, message *models.Message) {
	tapService := services.GetTapService()
	configs, err := tapService.GetAllTapTunnels()
	if err != nil {
		log.Printf("Error getting TAP Tunnel configs: %v", err)
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: "Error getting TAP Tunnel configs."}) 
		return
	}

	if len(configs) == 0 {
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: "No TAP Tunnel services configured."}) 
		return
	}

	var response strings.Builder
	response.WriteString("Configured TAP Tunnel Services:\n")
	for _, config := range configs {
		response.WriteString(fmt.Sprintf("\n- Name: %s\n", config.Name))
		response.WriteString(fmt.Sprintf("  Local IP: %s\n", config.LocalAddress))
		if config.MTU > 0 {
			response.WriteString(fmt.Sprintf("  MTU: %d\n", config.MTU))
		}
		response.WriteString(fmt.Sprintf("  Interface Name: %s\n", config.InterfaceName))
		response.WriteString(fmt.Sprintf("  Status: %s\n", config.Status))
	}

	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: message.Chat.ID,
		Text:   response.String(),
	})
}

func handleRemoveTap(ctx context.Context, b *bot.Bot, message *models.Message, args []string) {
	if len(args) != 1 {
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: "Usage: /remove_tap <name>"})
		return
	}
	name := args[0]
	tapService := services.GetTapService()
	config, err := tapService.GetTapTunnelByName(name)
	if err != nil {
		log.Printf("Error getting TAP Tunnel config by name %s: %v", name, err)
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: fmt.Sprintf("Config with name '%s' not found.", name)})
		return
	}

	if err := tapService.DeleteTapTunnel(config.ID); err != nil {
		log.Printf("Error deleting TAP Tunnel config %s: %v", name, err)
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: fmt.Sprintf("Error deleting config '%s': %v", name, err)})
		return
	}

	b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: fmt.Sprintf("TAP Tunnel config '%s' removed successfully.", name)})
}

func handleStartTap(ctx context.Context, b *bot.Bot, message *models.Message, args []string) {
	if len(args) != 1 {
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: "Usage: /start_tap <name>"})
		return
	}
	name := args[0]
	tapService := services.GetTapService()
	config, err := tapService.GetTapTunnelByName(name)
	if err != nil {
		log.Printf("Error getting TAP Tunnel config by name %s: %v", name, err)
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: fmt.Sprintf("Config with name '%s' not found.", name)})
		return
	}

	if config.Status == "up" {
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: fmt.Sprintf("TAP Tunnel '%s' is already running.", name)})
		return
	}

	// Re-create the tunnel
	if err := tapService.CreateTapTunnel(config); err != nil {
		log.Printf("Error starting TAP Tunnel %s: %v", name, err)
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: fmt.Sprintf("Error starting tunnel '%s': %v", name, err)})
		return
	}

	b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: fmt.Sprintf("TAP Tunnel '%s' started successfully.", name)})
}

func handleStopTap(ctx context.Context, b *bot.Bot, message *models.Message, args []string) {
	if len(args) != 1 {
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: "Usage: /stop_tap <name>"})
		return
	}
	name := args[0]
	tapService := services.GetTapService()
	config, err := tapService.GetTapTunnelByName(name)
	if err != nil {
		log.Printf("Error getting TAP Tunnel config by name %s: %v", name, err)
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: fmt.Sprintf("Config with name '%s' not found.", name)})
		return
	}

	if config.Status == "down" {
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: fmt.Sprintf("TAP Tunnel '%s' is not running.", name)})
		return
	}

	// Delete the tunnel to stop it
	if err := tapService.DeleteTapTunnel(config.ID); err != nil {
		log.Printf("Error stopping TAP Tunnel %s: %v", name, err)
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: fmt.Sprintf("Error stopping tunnel '%s': %v", name, err)})
		return
	}

	b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: fmt.Sprintf("TAP Tunnel '%s' stopped successfully.", name)})
}

// Subscription Domain Handlers
func handleSetSubDomain(ctx context.Context, b *bot.Bot, message *models.Message, args []string) {
	if len(args) != 1 {
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: "Usage: /set_sub_domain <domain>"})
		return
	}
	domain := args[0]
	settingService := services.GetConfigService().SettingService // Access SettingService via ConfigService
	if err := settingService.SetSubscriptionDomain(domain); err != nil {
		log.Printf("Error setting subscription domain: %v", err)
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: fmt.Sprintf("Error setting subscription domain: %v", err)})
		return
	}
	b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: fmt.Sprintf("Subscription domain set to: %s", domain)})
}

func handleGetSubDomain(ctx context.Context, b *bot.Bot, message *models.Message) {
	settingService := services.GetConfigService().SettingService // Access SettingService via ConfigService
	domain, err := settingService.GetWebDomain() // GetWebDomain now prioritizes subscriptionDomain
	if err != nil {
		log.Printf("Error getting subscription domain: %v", err)
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: fmt.Sprintf("Error getting subscription domain: %v", err)})
		return
	}
	if domain == "" {
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: "Subscription domain is not set. Using default web domain."}) 
	} else {
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: fmt.Sprintf("Current subscription domain: %s", domain)})
	}
}

// goudp2raw Tunnel Handlers
func handleAddUdp2rawClient(ctx context.Context, b *bot.Bot, message *models.Message, args []string) {
	if len(args) < 4 || len(args) > 5 {
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: "Usage: /add_udp2raw_client <name> <local_addr> <remote_server_ip> <key> [dscp]"})
		return
	}

	name := args[0]
	localAddr := args[1]
	remoteServerIP := args[2]
	key := args[3]
	dscp := 0
	if len(args) == 5 {
		var err error
		dscp, err = strconv.Atoi(args[4])
		if err != nil {
			b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: "Invalid DSCP value. It must be a number."}) 
			return
		}
	}

	config := model.Udp2rawConfig{
		Name:       name,
		Mode:       "client",
		LocalAddr:  localAddr,
		RemoteAddr: remoteServerIP,
		Key:        key,
		RawMode:    "icmp", // Default to ICMP for now
		DSCP:       dscp,
		Status:     "stopped",
	}

	udp2rawService := services.GetUdp2rawService()
	if err := udp2rawService.Save(&config); err != nil {
		log.Printf("Error creating goudp2raw client config: %v", err)
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: fmt.Sprintf("Error creating config: %v", err)})
		return
	}

	b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: fmt.Sprintf("goudp2raw client config '%s' created. Starting...", name)})

	if err := udp2rawService.Start(&config); err != nil {
		log.Printf("Error starting goudp2raw client: %v", err)
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: fmt.Sprintf("Error starting client: %v", err)})
		return
	}

	b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: fmt.Sprintf("goudp2raw client '%s' started successfully.", name)})
}

func handleAddUdp2rawServer(ctx context.Context, b *bot.Bot, message *models.Message, args []string) {
	if len(args) < 4 || len(args) > 5 {
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: "Usage: /add_udp2raw_server <name> <local_addr> <target_udp_service> <key> [dscp]"})
		return
	}

	name := args[0]
	localAddr := args[1]
	targetUdpService := args[2]
	key := args[3]
	dscp := 0
	if len(args) == 5 {
		var err error
		dscp, err = strconv.Atoi(args[4])
		if err != nil {
			b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: "Invalid DSCP value. It must be a number."}) 
			return
		}
	}

	config := model.Udp2rawConfig{
		Name:       name,
		Mode:       "server",
		LocalAddr:  localAddr,
		RemoteAddr: targetUdpService,
		Key:        key,
		RawMode:    "icmp", // Default to ICMP for now
		DSCP:       dscp,
		Status:     "stopped",
	}

	udp2rawService := services.GetUdp2rawService()
	if err := udp2rawService.Save(&config); err != nil {
		log.Printf("Error creating goudp2raw server config: %v", err)
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: fmt.Sprintf("Error creating config: %v", err)})
		return
	}

	b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: fmt.Sprintf("goudp2raw server config '%s' created. Starting...", name)})

	if err := udp2rawService.Start(&config); err != nil {
		log.Printf("Error starting goudp2raw server: %v", err)
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: fmt.Sprintf("Error starting server: %v", err)})
		return
	}

	b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: fmt.Sprintf("goudp2raw server '%s' started successfully.", name)})
}

func handleListUdp2raw(ctx context.Context, b *bot.Bot, message *models.Message) {
	udp2rawService := services.GetUdp2rawService()
	configs, err := udp2rawService.GetAll()
	if err != nil {
		log.Printf("Error getting goudp2raw configs: %v", err)
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: "Error getting goudp2raw configs."}) 
		return
	}

	if len(configs) == 0 {
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: "No goudp2raw tunnels configured."}) 
		return
	}

	var response strings.Builder
	response.WriteString("Configured goudp2raw Tunnels:\n")
	for _, config := range configs {
		response.WriteString(fmt.Sprintf("\n- Name: %s\n", config.Name))
		response.WriteString(fmt.Sprintf("  Mode: %s\n", config.Mode))
		response.WriteString(fmt.Sprintf("  Local Addr: %s\n", config.LocalAddr))
		response.WriteString(fmt.Sprintf("  Remote Addr: %s\n", config.RemoteAddr))
		response.WriteString(fmt.Sprintf("  Raw Mode: %s\n", config.RawMode))
		if config.DSCP > 0 {
			response.WriteString(fmt.Sprintf("  DSCP: %d\n", config.DSCP))
		}
		if len(config.Args) > 2 { // Check if it's not just "{}"
			response.WriteString(fmt.Sprintf("  Extra Args: %s\n", string(config.Args)))
		}
		response.WriteString(fmt.Sprintf("  Status: %s (PID: %d)\n", config.Status, config.PID))
	}

	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: message.Chat.ID,
		Text:   response.String(),
	})
}

func handleRemoveUdp2raw(ctx context.Context, b *bot.Bot, message *models.Message, args []string) {
	if len(args) != 1 {
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: "Usage: /remove_udp2raw <name>"})
		return
	}
	name := args[0]
	udp2rawService := services.GetUdp2rawService()
	config, err := udp2rawService.Get(name)
	if err != nil {
		log.Printf("Error getting goudp2raw config by name %s: %v", name, err)
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: fmt.Sprintf("Config with name '%s' not found.", name)})
		return
	}

	if config.PID != 0 {
		if err := udp2rawService.Stop(config); err != nil {
			log.Printf("Error stopping goudp2raw service %s during removal (will attempt deletion anyway): %v", name, err)
			b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: fmt.Sprintf("Tunnel '%s' was running, attempted to stop it before removal. It will be removed anyway.", name)})
		} else {
			b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: fmt.Sprintf("Tunnel '%s' stopped.", name)})
		}
	}

	if err := udp2rawService.Delete(name); err != nil {
		log.Printf("Error deleting goudp2raw config %s: %v", name, err)
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: fmt.Sprintf("Failed to delete config '%s': %v", name, err)})
		return
	}

	b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: fmt.Sprintf("goudp2raw config '%s' removed successfully.", name)})
}

func handleStartUdp2raw(ctx context.Context, b *bot.Bot, message *models.Message, args []string) {
	if len(args) != 1 {
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: "Usage: /start_udp2raw <name>"})
		return
	}
	name := args[0]
	udp2rawService := services.GetUdp2rawService()
	config, err := udp2rawService.Get(name)
	if err != nil {
		log.Printf("Error getting goudp2raw config by name %s: %v", name, err)
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: fmt.Sprintf("Config with name '%s' not found.", name)})
		return
	}

	if config.PID != 0 {
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: fmt.Sprintf("goudp2raw tunnel '%s' is already running.", name)})
		return
	}

	if err := udp2rawService.Start(config); err != nil {
		log.Printf("Error starting goudp2raw tunnel %s: %v", name, err)
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: fmt.Sprintf("Error starting tunnel '%s': %v", name, err)})
		return
	}

	b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: fmt.Sprintf("goudp2raw tunnel '%s' started successfully.", name)})
}

func handleStopUdp2raw(ctx context.Context, b *bot.Bot, message *models.Message, args []string) {
	if len(args) != 1 {
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: "Usage: /stop_udp2raw <name>"})
		return
	}
	name := args[0]
	udp2rawService := services.GetUdp2rawService()
	config, err := udp2rawService.Get(name)
	if err != nil {
		log.Printf("Error getting goudp2raw config by name %s: %v", name, err)
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: fmt.Sprintf("Config with name '%s' not found.", name)})
		return
	}

	if config.PID == 0 {
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: fmt.Sprintf("goudp2raw tunnel '%s' is not running.", name)})
		return
	}

	if err := udp2rawService.Stop(config); err != nil {
		log.Printf("Error stopping goudp2raw tunnel %s: %v", name, err)
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: fmt.Sprintf("Error stopping tunnel '%s': %v", name, err)})
		return
	}

	b.SendMessage(ctx, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: fmt.Sprintf("goudp2raw tunnel '%s' stopped successfully.", name)})
}