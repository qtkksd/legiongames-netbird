package hy2

// Hysteria2 client implementation.
//
// Currently using plain TCP as transport placeholder.
// The real Hysteria2 client will use:
//   github.com/apernet/hysteria/core/v2/client
//
// client.NewClient() returns:
//   type Client interface {
//       TCP(addr string) (net.Conn, error)
//       UDP() (HyUDPConn, error)
//       Close() error
//   }
//
// For WireGuard packet transport, UDP() provides a HyUDPConn
// with Receive/Send for datagram-based I/O.
// This is wrapped in the Hy2Tunnel bridge.
