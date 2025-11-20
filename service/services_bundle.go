package service

// ServicesBundle groups initialized service instances to pass between
// application components (app -> web -> api) without creating import cycles.
type ServicesBundle struct {
	SettingService   SettingService
	UserService      UserService
	ConfigService    *ConfigService
	ClientService    ClientService
	TlsService       TlsService
	InboundService   InboundService
	OutboundService  OutboundService
	EndpointService  EndpointService
	ServicesService  ServicesService
	PanelService     PanelService
	StatsService     StatsService
	ServerService    ServerService
	ChiselService    *ChiselService
	GostService      *GostService
	TapService       *TapService
	UdpTunnelService *UdpTunnelService
}
