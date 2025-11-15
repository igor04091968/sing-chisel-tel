package model

import "gorm.io/gorm"

type ChiselConfig struct {
	gorm.Model
	Name          string `gorm:"unique" json:"tag"`
	Mode          string `json:"mode"` // "client" or "server"
	ServerAddress string `json:"server_address"`
	ServerPort    int    `json:"server_port"`
	ListenAddress string `json:"listen_address"`
	ListenPort    int    `json:"listen_port"`
	Args          string `json:"args"`
	PID           int    `json:"-"`
}
