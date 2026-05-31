package appui

import (
	"fmt"
	"net"
	"os/exec"
	"runtime"
	"strings"
)

func highlightURL(url string) string {
	return "\033[36m" + url + "\033[0m"
}

func pickServePort(preferredPort, maxTries int) (net.Listener, int, error) {
	for i := 0; i < maxTries; i++ {
		candidate := preferredPort + i

		if runtime.GOOS == "windows" {
			lnLocal, errLocal := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", candidate))
			lnAll, errAll := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", candidate))

			if errLocal != nil || errAll != nil {
				if lnLocal != nil {
					lnLocal.Close()
				}
				if lnAll != nil {
					lnAll.Close()
				}
				continue
			}

			lnAll.Close()
			return lnLocal, candidate, nil
		}

		ln, err := net.Listen("tcp", fmt.Sprintf(":%d", candidate))
		if err == nil {
			return ln, candidate, nil
		}
	}

	return nil, 0, fmt.Errorf("no available port found starting from %d", preferredPort)
}

func openURL(url string) error {
	var cmd string
	var args []string

	switch runtime.GOOS {
	case "windows":
		cmd = "rundll32"
		args = []string{"url.dll,FileProtocolHandler", url}
	case "darwin":
		cmd = "open"
		args = []string{url}
	default:
		if isWSL() {
			cmd = "cmd.exe"
			args = []string{"/c", "start", url}
		} else {
			cmd = "xdg-open"
			args = []string{url}
		}
	}
	return exec.Command(cmd, args...).Start()
}

func isWSL() bool {
	releaseData, err := exec.Command("uname", "-r").Output()
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(string(releaseData)), "microsoft")
}
