package hy2

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
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
	m.started = false
	return nil
}

// CreateTunnel establishes a Hy2 P2P transport over the ICE hole-punched connection.
// iceConn is the ICE agents Dial() or Accept() result -- a bidirectional UDP path
// through NAT that does not require port forwarding.
func (m *Manager) CreateTunnel(ctx context.Context, peerKey string, overlayIP netip.Addr, iceConn net.Conn) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.ctx == nil || m.ctx.Err() != nil {
		return fmt.Errorf("Hy2 manager not started")
	}
	if _, exists := m.tunnels[peerKey]; exists {
		log.Debugf("Hy2 tunnel for peer %s already exists", peerKey)
		return nil
	}
	tunnel := &Hy2Tunnel{conn: iceConn, peerKey: peerKey}
	go tunnel.bridgeToWG(m.ctx, m.injector, overlayIP)
	m.tunnels[peerKey] = tunnel
	log.Infof("Hy2 tunnel created for peer %s via ICE path", peerKey)
	return nil
}

type Hy2Tunnel struct {
	conn    net.Conn
	peerKey string
}

// bridgeToWG bridges packets between the ICE connection and the WireGuard device.
// The ICE connection is a hole-punched UDP path. Each Write() on it sends a datagram
// to the peer; each Read() receives one. We add a 2-byte length prefix for framing.
func (t *Hy2Tunnel) bridgeToWG(ctx context.Context, injector EndpointInjector, overlayIP netip.Addr) {
	defer t.conn.Close()

	// Register for outbound: when WG sends to overlayIP, ICEBind routes through this conn.
	injector.SetEndpoint(overlayIP, t.conn)
	defer injector.RemoveEndpoint(overlayIP)

	// Inbound: read from ICE conn, inject into WG via ReceiveFromEndpoint.
	buf := make([]byte, 2048)
	for {
		n, err := readFrame(t.conn, buf)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Debugf("Hy2 tunnel read error for peer %s: %v", t.peerKey, err)
			return
		}
		if n > 0 {
			// Use a dummy endpoint -- WireGuard identifies the peer by the overlay IP
			// mapped via SetEndpoint, not by this endpoint address.
			ep := &bind.Endpoint{AddrPort: netip.MustParseAddrPort("127.0.0.1:0")}
			injector.ReceiveFromEndpoint(ctx, ep, buf[:n])
		}
	}
}

// readFrame reads a length-prefixed frame from the connection.
// WireGuard packets vary in size; the 2-byte length prefix preserves
// packet boundaries over the stream-oriented ICE connection.
func readFrame(r io.Reader, buf []byte) (int, error) {
	var lenBuf [2]byte
	if _, err := io.ReadFull(r, lenBuf[:]); err != nil {
		return 0, err
	}
	dataLen := binary.BigEndian.Uint16(lenBuf[:])
	if int(dataLen) > len(buf) {
		return 0, fmt.Errorf("frame too large: %d > %d", dataLen, len(buf))
	}
	return io.ReadFull(r, buf[:dataLen])
}

func (t *Hy2Tunnel) Close() error {
	return t.conn.Close()
}
