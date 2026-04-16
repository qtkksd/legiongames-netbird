package version

import (
	"golang.org/x/sys/windows/registry"
	"runtime"
)

const (
	urlWinExe    = "https://pkgs.legiongames.ru/windows/x64/netbird-setup.exe"
	urlWinExeArm = "https://pkgs.legiongames.ru/windows/arm64/netbird-setup.exe"
)

var regKeyAppPath = "SOFTWARE\\Microsoft\\Windows\\CurrentVersion\\App Paths\\Netbird"

// DownloadUrl return with the proper download link
func DownloadUrl() string {
	_, err := registry.OpenKey(registry.LOCAL_MACHINE, regKeyAppPath, registry.QUERY_VALUE)
	if err != nil {
		return downloadURL
	}

	url := urlWinExe
	if runtime.GOARCH == "arm64" {
		url = urlWinExeArm
	}

	return url
}
