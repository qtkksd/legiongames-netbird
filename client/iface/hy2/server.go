package hy2

import (
	"context"
	"net"
	"net/netip"
	"sync"
)

type Hy2Server struct {
	ctx      context.Context
	cancel   context.CancelFunc
	config   *Config
	manager  *Manager
	listener net.Listener
	wg       sync.WaitGroup
}

func NewHy2Server(ctx context.Context, config *Config, manager *Manager) (*Hy2Server, error) {
	srvCtx, cancel := context.WithCancel(ctx)
	listener, err := net.Listen("tcp", config.ServerListenAddr)
	if err != nil {
		cancel()
		return nil, err
	}
	s := &Hy2Server{
		ctx:      srvCtx,
		cancel:   cancel,
		config:   config,
		manager:  manager,
		listener: listener,
	}
	s.wg.Add(1)
	go s.acceptLoop()
	return s, nil
}

func (s *Hy2Server) acceptLoop() {
	defer s.wg.Done()
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.ctx.Done():
				return
			default:
				log.Warnf("Hy2 server accept error: %v", err)
				return
			}
		}
		go s.handleConnection(conn)
	}
}

func (s *Hy2Server) handleConnection(conn net.Conn) {
	peerKeyBuf := make([]byte, 64)
	n, err := conn.Read(peerKeyBuf)
	if err != nil || n == 0 {
		conn.Close()
		return
	}
	peerKey := string(peerKeyBuf[:n])
	overlayIP := net.IP{127, 0, 0, 1}
	addr, _ := netip.AddrFromSlice(overlayIP)
	s.manager.onIncomingTunnel(s.ctx, peerKey, addr, conn)
}

func (s *Hy2Server) Close() error {
	s.cancel()
	s.listener.Close()
	s.wg.Wait()
	return nil
}
