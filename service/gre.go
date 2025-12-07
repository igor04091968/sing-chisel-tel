package service

import (
	"fmt"
	"net"

	"github.com/igor04091968/sing-chisel-tel/database"
	"github.com/igor04091968/sing-chisel-tel/database/model"
	"github.com/vishvananda/netlink"
	"gorm.io/gorm"
)

// GreService handles the business logic for GRE tunnels.
// NOTE: All methods that manipulate network interfaces require root privileges to run.
type GreService struct {
	db *gorm.DB
}

// NewGreService creates a new instance of GreService.
func NewGreService() *GreService {
	return &GreService{
		db: database.GetDB(),
	}
}

// CreateGreTunnel creates a new GRE tunnel interface and saves its config to the DB.
// This operation requires root privileges.
func (s *GreService) CreateGreTunnel(config *model.GreTunnel) error {
	localIP := net.ParseIP(config.LocalAddress)
	if localIP == nil {
		return fmt.Errorf("invalid local address: %s", config.LocalAddress)
	}

	remoteIP := net.ParseIP(config.RemoteAddress)
	if remoteIP == nil {
		return fmt.Errorf("invalid remote address: %s", config.RemoteAddress)
	}

	// Define the GRE tunnel interface
	greTunnel := &netlink.Gretun{
		LinkAttrs: netlink.LinkAttrs{
			Name: config.Name,
		},
		Local:  localIP,
		Remote: remoteIP,
	}

	// Add the tunnel interface
	if err := netlink.LinkAdd(greTunnel); err != nil {
		return fmt.Errorf("failed to add GRE tunnel interface '%s': %w", config.Name, err)
	}

	// Parse the tunnel address
	addr, err := netlink.ParseAddr(config.TunnelAddress)
	if err != nil {
		_ = netlink.LinkDel(greTunnel) // Rollback
		return fmt.Errorf("invalid tunnel address '%s': %w", config.TunnelAddress, err)
	}

	// Add the address to the tunnel interface
	if err := netlink.AddrAdd(greTunnel, addr); err != nil {
		_ = netlink.LinkDel(greTunnel) // Rollback
		return fmt.Errorf("failed to add address to tunnel '%s': %w", config.Name, err)
	}

	// Bring the tunnel interface up
	if err := netlink.LinkSetUp(greTunnel); err != nil {
		_ = netlink.LinkDel(greTunnel) // Rollback
		return fmt.Errorf("failed to bring up tunnel '%s': %w", config.Name, err)
	}

	// Save to database
	config.Status = "up"
	if err := s.db.Create(config).Error; err != nil {
		_ = s.DeleteGreTunnel(config.ID) // Rollback from DB and network
		return fmt.Errorf("failed to save GRE tunnel config to database: %w", err)
	}

	return nil
}

// DeleteGreTunnel deletes a GRE tunnel interface and removes its config from the DB.
// This operation requires root privileges.
func (s *GreService) DeleteGreTunnel(id uint) error {
	// First, find the config in the DB
	config, err := s.GetGreTunnel(id)
	if err != nil {
		return fmt.Errorf("failed to find GRE tunnel with ID %d: %w", id, err)
	}

	// Find the link by name
	link, err := netlink.LinkByName(config.Name)
	if err != nil {
		// If link doesn't exist, we can still proceed to delete from DB
		// but we should log it.
		fmt.Printf("Warning: could not find link '%s' to delete, but proceeding with DB record removal. Error: %v\n", config.Name, err)
	} else {
		// If link exists, delete it
		if err := netlink.LinkDel(link); err != nil {
			return fmt.Errorf("failed to delete GRE tunnel interface '%s': %w", config.Name, err)
		}
	}

	// Delete from database
	if err := s.db.Delete(&model.GreTunnel{}, id).Error; err != nil {
		return fmt.Errorf("failed to delete GRE tunnel config from database: %w", err)
	}

	return nil
}

// GetGreTunnelByName retrieves a single GRE tunnel configuration by its name.
func (s *GreService) GetGreTunnelByName(name string) (*model.GreTunnel, error) {
	var config model.GreTunnel
	err := s.db.Where("name = ?", name).First(&config).Error
	return &config, err
}

// GetAllGreTunnels retrieves all GRE tunnel configurations from the database.
func (s *GreService) GetAllGreTunnels() ([]model.GreTunnel, error) {
	var configs []model.GreTunnel
	err := s.db.Find(&configs).Error
	return configs, err
}

// GetGreTunnel retrieves a single GRE tunnel configuration by its ID.
func (s *GreService) GetGreTunnel(id uint) (*model.GreTunnel, error) {
	var config model.GreTunnel
	err := s.db.First(&config, id).Error
	return &config, err
}
