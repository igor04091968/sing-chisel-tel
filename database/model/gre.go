package model

import "gorm.io/gorm"

// GreTunnel represents the configuration for a GRE tunnel.
type GreTunnel struct {
	gorm.Model
	Name          string `gorm:"unique" json:"name"`           // Name of the tunnel interface, e.g., "gre1"
	LocalAddress  string `json:"local_address"`                // Local physical IP address
	RemoteAddress string `json:"remote_address"`               // Remote physical IP address
	TunnelAddress string `json:"tunnel_address"`               // IP address and mask for the tunnel itself, e.g., "10.0.0.1/30"
	InterfaceName string `json:"interface_name"`               // User-defined name for the interface
	Status        string `json:"status" gorm:"default:'down'"` // Status of the tunnel, e.g., "up" or "down"
}
