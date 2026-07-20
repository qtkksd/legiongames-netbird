//go:build !linux || android

package device

import "github.com/netbirdio/netbird/client/internal/amneziawg"

// WireGuardModuleIsLoaded reports whether the kernel WireGuard module is available.
func WireGuardModuleIsLoaded(_ amneziawg.AmneziaConfig) bool {
	return false
}

// ModuleTunIsLoaded reports whether the tun device is available.
func ModuleTunIsLoaded() bool {
	return true
}
