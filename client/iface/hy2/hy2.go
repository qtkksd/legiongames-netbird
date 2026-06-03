package hy2

import (
	"context"
	"net"
	"net/netip"

	"github.com/netbirdio/netbird/client/iface/bind"
	"github.com/sirupsen/logrus"
)

var log = logrus.WithField("package", "hy2")

// Config holds Hysteria2 configuration.
type Config struct {
	ServerEnabled    bool   // true for Pi peers
	ServerListenAddr string // e.g., ":4433"
	ServerCertFile   string // TLS cert path
	ServerKeyFile    string // TLS key path
	MasqueradeSNI    string // SNI for HTTP/3 masquerade, e.g., "cloudflare.com"
	MasqueradePort   int    // port for masquerade target, default 443
}

// EndpointInjector is the full ICEBind / RelayBindJS interface
// needed to route WG packets through a proxied connection.
type EndpointInjector interface {
	SetEndpoint(fakeIP netip.Addr, conn net.Conn)
	RemoveEndpoint(fakeIP netip.Addr)
	ReceiveFromEndpoint(ctx context.Context, ep *bind.Endpoint, buf []byte)
}
