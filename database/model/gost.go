package model

import "gorm.io/gorm"

// GostConfig represents configuration for a gost reverse tunnel instance.
type GostConfig struct {
    gorm.Model
    Name          string `gorm:"unique" json:"name"`
    Mode          string `json:"mode"` // "client" or "server"
    ServerAddress string `json:"server_address"`
    ServerPort    int    `json:"server_port"`
    ListenAddress string `json:"listen_address"`
    ListenPort    int    `json:"listen_port"`
    Args          string `json:"args"`              // Raw extra args passed to gost
    Status        string `json:"status" gorm:"default:'down'"`
    PID           int    `json:"-"`
}
