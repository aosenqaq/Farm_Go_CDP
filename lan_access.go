package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"sync"

	"Farm_Go/internal/lanaccess"
	"Farm_Go/internal/storage"
)

const (
	ModeLAN    = lanaccess.ModeLAN
	ModeTunnel = lanaccess.ModeTunnel
)

// LANAccessSettingsInput is the desktop-only Wails boundary that accepts a password.
type LANAccessSettingsInput struct {
	Enabled         bool   `json:"enabled"`
	Mode            string `json:"mode"`
	Port            int    `json:"port"`
	Password        string `json:"password"`
	ConfirmPassword string `json:"confirmPassword"`
}

// LANAccessStatus deliberately excludes the password hash.
type LANAccessStatus struct {
	Enabled            bool   `json:"enabled"`
	Mode               string `json:"mode"`
	Port               int    `json:"port"`
	PasswordConfigured bool   `json:"passwordConfigured"`
	Running            bool   `json:"running"`
	Address            string `json:"address"`
	Error              string `json:"error"`
}

type lanAccessManagerOptions struct {
	Assets     fs.FS
	GameAssets http.Handler
	Poll       http.Handler
	RPC        lanaccess.RPCDispatcher
	Save       func(context.Context, storage.LANAccessSettings) error
}

type lanAccessManager struct {
	mu sync.Mutex

	assets     fs.FS
	gameAssets http.Handler
	poll       http.Handler
	rpc        lanaccess.RPCDispatcher
	save       func(context.Context, storage.LANAccessSettings) error

	settings storage.LANAccessSettings
	service  *lanAccessService
	lastErr  string
}

type lanAccessService struct {
	settings storage.LANAccessSettings
	auth     *lanaccess.Authenticator
	listener net.Listener
	server   *http.Server
}

func newLANAccessManager(options lanAccessManagerOptions) *lanAccessManager {
	return &lanAccessManager{
		assets:     lanAccessAssets(options.Assets),
		gameAssets: options.GameAssets,
		poll:       options.Poll,
		rpc:        options.RPC,
		save:       options.Save,
		settings:   storage.DefaultLANAccessSettings(),
	}
}

func lanAccessAssets(assets fs.FS) fs.FS {
	if assets == nil {
		return nil
	}
	sub, err := fs.Sub(assets, "frontend/dist")
	if err != nil {
		return assets
	}
	return sub
}

func (a *App) LANAccessSettings() LANAccessStatus {
	if a.lanAccess == nil {
		return lanAccessStatus(storage.DefaultLANAccessSettings(), nil, "")
	}
	return a.lanAccess.Status()
}

func (a *App) SaveLANAccessSettings(input LANAccessSettingsInput) (LANAccessStatus, error) {
	if a.lanAccess == nil {
		err := errors.New("LAN access manager is not available")
		return LANAccessStatus{}, err
	}

	current := a.lanAccess.Settings()
	settings, err := lanAccessSettingsFromInput(input, current)
	if err != nil {
		a.lanAccess.SetError(err)
		return a.lanAccess.Status(), err
	}
	if err := a.lanAccess.Apply(a.contextOrBackground(), settings); err != nil {
		return a.lanAccess.Status(), err
	}
	return a.lanAccess.Status(), nil
}

func lanAccessSettingsFromInput(input LANAccessSettingsInput, current storage.LANAccessSettings) (storage.LANAccessSettings, error) {
	if input.Password != input.ConfirmPassword {
		return storage.LANAccessSettings{}, errors.New("LAN access password confirmation does not match")
	}

	passwordHash := current.PasswordHash
	if input.Password != "" {
		var err error
		passwordHash, err = lanaccess.HashPassword(input.Password)
		if err != nil {
			return storage.LANAccessSettings{}, err
		}
	}
	settings := storage.LANAccessSettings{
		Enabled:      input.Enabled,
		Mode:         storage.LANAccessMode(input.Mode),
		Port:         input.Port,
		PasswordHash: passwordHash,
	}
	if settings.Enabled && settings.PasswordHash == "" {
		return storage.LANAccessSettings{}, errors.New("LAN access password is required")
	}
	return settings, nil
}

func (m *lanAccessManager) Settings() storage.LANAccessSettings {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.settings
}

func (m *lanAccessManager) Status() LANAccessStatus {
	m.mu.Lock()
	defer m.mu.Unlock()
	return lanAccessStatus(m.settings, m.service, m.lastErr)
}

func lanAccessStatus(settings storage.LANAccessSettings, service *lanAccessService, lastErr string) LANAccessStatus {
	status := LANAccessStatus{
		Enabled:            settings.Enabled,
		Mode:               string(settings.Mode),
		Port:               settings.Port,
		PasswordConfigured: settings.PasswordHash != "",
		Error:              lastErr,
	}
	if service != nil {
		status.Running = true
		status.Address = lanAccessDisplayAddress(lanaccess.Mode(settings.Mode), settings.Port, localLANIPv4Addresses())
	}
	return status
}

func lanAccessDisplayAddress(mode lanaccess.Mode, port int, addresses []netip.Addr) string {
	if mode == lanaccess.ModeTunnel {
		return net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
	}
	for _, address := range addresses {
		if address.Is4() && address.IsPrivate() {
			return net.JoinHostPort(address.String(), strconv.Itoa(port))
		}
	}
	return ""
}

func localLANIPv4Addresses() []netip.Addr {
	interfaceAddresses, err := net.InterfaceAddrs()
	if err != nil {
		return nil
	}
	addresses := make([]netip.Addr, 0, len(interfaceAddresses))
	for _, interfaceAddress := range interfaceAddresses {
		ip, _, err := net.ParseCIDR(interfaceAddress.String())
		if err != nil {
			continue
		}
		address, ok := netip.AddrFromSlice(ip)
		if ok {
			addresses = append(addresses, address.Unmap())
		}
	}
	return addresses
}

func (m *lanAccessManager) SetError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err == nil {
		m.lastErr = ""
		return
	}
	m.lastErr = err.Error()
}

func (m *lanAccessManager) Restore(settings storage.LANAccessSettings) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !settings.Enabled {
		m.settings = settings
		m.lastErr = ""
		return nil
	}
	candidate, err := m.newService(settings)
	if err != nil {
		m.settings = settings
		m.lastErr = err.Error()
		return err
	}
	old := m.service
	m.service = candidate
	m.settings = settings
	m.lastErr = ""
	if old != nil {
		return old.Close()
	}
	return nil
}

// Apply preserves the active service until a candidate listener is live and its
// configuration has been persisted.
func (m *lanAccessManager) Apply(ctx context.Context, settings storage.LANAccessSettings) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := lanAccessConfig(settings).Validate(); err != nil {
		m.lastErr = err.Error()
		return err
	}

	if !settings.Enabled {
		if err := m.persist(ctx, settings); err != nil {
			m.lastErr = err.Error()
			return err
		}
		old := m.service
		m.service = nil
		m.settings = settings
		m.lastErr = ""
		if old != nil {
			if err := old.Close(); err != nil {
				m.lastErr = err.Error()
				return err
			}
		}
		return nil
	}

	if m.service != nil && lanAccessConfig(m.service.settings).ListenAddr() == lanAccessConfig(settings).ListenAddr() {
		if err := m.persist(ctx, settings); err != nil {
			m.lastErr = err.Error()
			return err
		}
		m.service.auth.SetPasswordHash(settings.PasswordHash)
		m.service.settings = settings
		m.settings = settings
		m.lastErr = ""
		return nil
	}

	candidate, err := m.newService(settings)
	if err != nil {
		m.lastErr = err.Error()
		return err
	}
	if err := m.persist(ctx, settings); err != nil {
		_ = candidate.Close()
		m.lastErr = err.Error()
		return err
	}

	old := m.service
	m.service = candidate
	m.settings = settings
	m.lastErr = ""
	if old != nil {
		if err := old.Close(); err != nil {
			m.lastErr = err.Error()
			return err
		}
	}
	return nil
}

func (m *lanAccessManager) persist(ctx context.Context, settings storage.LANAccessSettings) error {
	if m.save == nil {
		return errors.New("LAN access storage is not available")
	}
	if err := m.save(ctx, settings); err != nil {
		return fmt.Errorf("save LAN access settings: %w", err)
	}
	return nil
}

func (m *lanAccessManager) newService(settings storage.LANAccessSettings) (*lanAccessService, error) {
	config := lanAccessConfig(settings)
	if err := config.Validate(); err != nil {
		return nil, err
	}
	listener, err := net.Listen("tcp", config.ListenAddr())
	if err != nil {
		return nil, fmt.Errorf("listen on %s: %w", config.ListenAddr(), err)
	}
	auth := lanaccess.NewAuthenticator(nil)
	auth.SetPasswordHash(settings.PasswordHash)
	server := &http.Server{Handler: lanaccess.NewServer(lanaccess.Options{
		Config:     config,
		Auth:       auth,
		Assets:     m.assets,
		GameAssets: m.gameAssets,
		RPC:        m.rpc,
		Poll:       m.poll,
	})}
	service := &lanAccessService{
		settings: settings,
		auth:     auth,
		listener: listener,
		server:   server,
	}
	go func() { _ = server.Serve(listener) }()
	return service, nil
}

func lanAccessConfig(settings storage.LANAccessSettings) lanaccess.Config {
	return lanaccess.Config{
		Enabled:      settings.Enabled,
		Mode:         lanaccess.Mode(settings.Mode),
		Port:         settings.Port,
		PasswordHash: settings.PasswordHash,
	}
}

func (m *lanAccessManager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.service == nil {
		return nil
	}
	service := m.service
	m.service = nil
	if err := service.Close(); err != nil {
		m.lastErr = err.Error()
		return err
	}
	return nil
}

func (s *lanAccessService) Close() error {
	err := s.server.Close()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}
