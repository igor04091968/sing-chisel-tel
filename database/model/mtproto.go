package model

import "gorm.io/gorm"

// MTProtoProxyConfig represents the configuration for an MTProto Proxy.
type MTProtoProxyConfig struct {
	gorm.Model
	Name       string `gorm:"unique" json:"name"`           // Name of the MTProto Proxy instance
	ListenPort int    `json:"listen_port"`                  // Port on which the proxy will listen
	Secret     string `json:"secret"`                       // The MTProto secret (32-byte hex string)
	AdTag      string `json:"ad_tag,omitempty"`             // Optional AdTag for promoting Telegram channels
	Status     string `json:"status" gorm:"default:'down'"` // Status of the proxy, e.g., "up" or "down"
}
