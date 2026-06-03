package hy2

// server.go
//
// The Hy2 "server" does not listen on a separate port.
// Instead, Hy2 rides on the ICE hole-punched connection
// that is already established between peers.
//
// Both peers call CreateTunnel with the ICE RemoteConn
// after ICE succeeds. No port forwarding required.
//
// Future: when Hysteria2 QUIC library is integrated,
// the QUIC handshake will be negotiated over this same
// ICE connection path.
