package notification

type ChannelConfig struct {
	Type   string   `json:"type"`
	Events []string `json:"events"`

	// Telegram
	BotToken string `json:"bot_token,omitempty"`
	ChatID   string `json:"chat_id,omitempty"`

	// Webhook
	URL string `json:"url,omitempty"`
}

type ProjectConfig struct {
	Project  string          `json:"project"`
	Channels []ChannelConfig `json:"channels"`
}
