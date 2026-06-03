package hy2

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"sync"

	"github.com/netbirdio/netbird/client/iface/bind"
	"github.com/netbirdio/netbird/client/iface/device"
)

type WGIface interface {
	GetBind() device.EndpointManager
	IsUserspaceBind() bool
}

type Manager struct {
	config   *Config
	injector EndpointInjector
	wgIface  WGIface

	mu      sync.Mutex
	server  *Hy2Server
	tunnels map[string]*Hy2Tunnel
	ctx     context.Context
	cancel  context.CancelFunc
	started bool
}

func NewManager(config *Config, wgIface WGIface) (*Manager, error) {
	bind := wgIface.GetBind()
	if bind == nil {
		return nil, fmt.Errorf("Hy2 requires userspace WireGuard bind")
	}
	injector, ok := bind.(EndpointInjector)
	if !ok {
		return nil, fmt.Errorf("bind does not implement EndpointInjector")
	}
	return &Manager{
		config:   config,
		injector: injector,
		wgIface:  wgIface,
		tunnels:  make(map[string]*Hy2Tunnel),
	}, nil
}

func (m *Manager) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.started {
		return nil
	}
	m.ctx, m.cancel = context.WithCancel(ctx)
	if m.config.ServerEnabled {
		server, err := NewHy2Server(m.ctx, m.config, m)
		if err != nil {
			return fmt.Errorf("start Hy2 server: %w", err)
		}
		m.server = server
		log.Infof("Hy2 server started on %s", m.config.ServerListenAddr)
	}
	m.started = true
	return nil
}

func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.started {
		return nil
	}
	if m.cancel != nil {
		m.cancel()
	}
	for key, t := range m.tunnels {
		if err := t.Close(); err != nil {
			log.Warnf("Hy2 tunnel close error for %s: %v", key, err)
		}
	}
	m.tunnels = make(map[string]*Hy2Tunnel)
	if m.server != nil {
		if err := m.server.Close(); err != nil {
			log.Warnf("Hy2 server close error: %v", err)
		}
		m.server = nil
	}
	m.started = false
	return nil
}

func (m *Manager) CreateTunnel(ctx context.Context, peerKey string, overlayIP netip.Addr, peerAddr *net.UDPAddr) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.ctx == nil || m.ctx.Err() != nil {
		return fmt.Errorf("Hy2 manager not started")
	}
	if _, exists := m.tunnels[peerKey]; exists {
		log.Debugf("Hy2 tunnel for peer %s already exists", peerKey)
		return nil
	}
	hy2Addr := fmt.Sprintf("%s:%d", peerAddr.IP.String(), 4433)
	tunnel, err := NewHy2Tunnel(ctx, hy2Addr, peerKey)
	if err != nil {
		return fmt.Errorf("create Hy2 tunnel: %w", err)
	}
	go tunnel.bridgeToWG(m.ctx, m.injector, overlayIP)
	m.tunnels[peerKey] = tunnel
	log.Infof("Hy2 tunnel created for peer %s via %s", peerKey, hy2Addr)
	return nil
}

func (m *Manager) onIncomingTunnel(ctx context.Context, peerKey string, overlayIP netip.Addr, conn net.Conn) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.ctx == nil || m.ctx.Err() != nil {
		conn.Close()
		return
	}
	if _, exists := m.tunnels[peerKey]; exists {
		conn.Close()
		return
	}
	tunnel := &Hy2Tunnel{conn: conn, peerKey: peerKey}
	go tunnel.bridgeToWG(m.ctx, m.injector, overlayIP)
	m.tunnels[peerKey] = tunnel
	log.Infof("Hy2 tunnel accepted for incoming peer %s", peerKey)
}

type Hy2Tunnel struct {
	conn    net.Conn
	peerKey string
}

func NewHy2Tunnel(ctx context.Context, addr string, peerKey string) (*Hy2Tunnel, error) {
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("dial Hy2 peer %s: %w", addr, err)
	}
	return &Hy2Tunnel{conn: conn, peerKey: peerKey}, nil
}

func (t *Hy2Tunnel) bridgeToWG(ctx context.Context, injector EndpointInjector, overlayIP netip.Addr) {
	defer t.conn.Close()
	injector.SetEndpoint(overlayIP, t.conn)
	defer injector.RemoveEndpoint(overlayIP)
	buf := make([]byte, 2048)
	for {
		n, err := t.conn.Read(buf)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Debugf("Hy2 tunnel read error: %v", err)
			return
		}
		if n > 0 {
			ep := &bind.Endpoint{AddrPort: netip.MustParseAddrPort("127.0.0.1:0")}
			injector.ReceiveFromEndpoint(ctx, ep, buf[:n])
		}
	}
}

func (t *Hy2Tunnel) Close() error {
	return t.conn.Close()
}
