package model

import (
	"gorm.io/gorm"
)

// UdpTunnelConfig represents the configuration for a UDP tunnel with udp2raw-like features.
type UdpTunnelConfig struct {
	gorm.Model
	Name                string `json:"name" gorm:"uniqueIndex"`
	Role                string `json:"role"`                   // "client" or "server"
	ListenPort          int    `json:"listen_port"`            // For client: where local app sends UDP. For server: where udp2raw server listens.
	RemoteAddress       string `json:"remote_address"`         // For client: where to send faketcp/udp2raw. For server: where to forward decoded UDP payload.
	Mode                string `json:"mode"`           // e.g., "faketcp", "icmp", "raw_udp"
	VLANID              uint16 `json:"vlan_id,omitempty"`       // 802.1Q VLAN ID
	VLANPriority        uint8  `json:"vlan_priority,omitempty"` // 802.1p Priority Code Point (0-7)
	DSCP                uint8  `json:"dscp,omitempty"`          // DiffServ Code Point (0-63)
	InterfaceName       string `json:"interface_name,omitempty"` // e.g., "eth0" for --lower-level
	DestMAC             string `json:"dest_mac,omitempty"`       // Destination MAC address for --lower-level
	FakeTCPFlags        string `json:"fake_tcp_flags,omitempty"` // e.g., "SYN", "SYNACK"
	Status              string `json:"status"`                   // "running", "stopped"
	ProcessID           int    `json:"process_id,omitempty"`     // Placeholder for internal process management
	// Additional fields for server-side. ServerListenAddress is not needed as RemoteAddress specifies the forward target.
}
