package model

import "gorm.io/gorm"

type ChiselConfig struct {
	gorm.Model
	Name          string `gorm:"unique"`
	Mode          string // "client" or "server"
	ServerAddress string
	ServerPort    int
	ListenAddress string
	ListenPort    int
	Args          string
	PID           int
}
