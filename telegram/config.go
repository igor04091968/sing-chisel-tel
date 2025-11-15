package telegram

type Config struct {
	BotToken     string  `json:"bot_token"`
	AdminUserIDs []int64 `json:"admin_user_ids"`
	Enabled      bool    `json:"enabled"`
}
