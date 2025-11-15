package model

import "gorm.io/gorm"

// TapTunnel represents the configuration for a TAP tunnel.
type TapTunnel struct {
	gorm.Model
	Name          string `gorm:"unique" json:"name"`           // Name of the TAP interface, e.g., "tap0"
	LocalAddress  string `json:"local_address"`                // IP address and mask for the TAP interface, e.g., "192.168.50.1/24"
	MTU           int    `json:"mtu" gorm:"default:1500"`      // MTU for the TAP interface
	InterfaceName string `json:"interface_name"`               // User-defined name for the interface
	Status        string `json:"status" gorm:"default:'down'"` // Status of the tunnel, e.g., "up" or "down"
}
