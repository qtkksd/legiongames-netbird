package version

import (
	"golang.org/x/sys/windows/registry"
	"runtime"
)

const (
	urlWinExe    = "https://pkgs.legiongames.ru/netbird/latest/windows/x64/netbird.exe"
	urlWinExeArm = "https://pkgs.legiongames.ru/netbird/latest/windows/arm64/netbird.exe"
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
