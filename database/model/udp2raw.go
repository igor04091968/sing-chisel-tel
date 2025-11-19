package model

import (
	"encoding/json"

	"gorm.io/gorm"
)

// Udp2rawConfig represents a udp2raw tunnel configuration.
type Udp2rawConfig struct {
	ID         uint   `gorm:"primaryKey"`
	Name       string `gorm:"unique;not null"`
	Mode       string `gorm:"not null"` // "server" or "client"
	LocalAddr  string // Address for local endpoint
	RemoteAddr string // Address for remote endpoint
	Key        string // Pre-shared key
	RawMode    string // "icmp", "udp", "faketcp"
	DSCP       int    // DSCP value for QoS
	Args       json.RawMessage // Extra command-line arguments as JSON
	Status     string `gorm:"default:'stopped'"`
	PID        int    `gorm:"default:0"`
	Remark     string
}

func (u *Udp2rawConfig) TableName() string {
	return "udp2raw_configs"
}

func (u *Udp2rawConfig) BeforeCreate(tx *gorm.DB) (err error) {
	if u.Status == "" {
		u.Status = "stopped"
	}
	if u.Args == nil {
		u.Args = json.RawMessage(`{}`)
	}
	return
}
