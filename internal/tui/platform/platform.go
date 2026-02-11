package platform

import (
	"bytes"
	"fmt"
	"net/url"
	"os/exec"
	"runtime"
	"strings"
)

func ValidateEntryURL(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", fmt.Errorf("entry has no URL")
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "", fmt.Errorf("invalid URL format")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("unsupported URL scheme: %s", parsed.Scheme)
	}
	if parsed.Host == "" {
		return "", fmt.Errorf("invalid URL host")
	}
	return trimmed, nil
}

func OpenURLInBrowser(url string) error {
	name, args := browserCommand(runtime.GOOS, url)
	cmd := exec.Command(name, args...)
	return cmd.Run()
}

func CopyURLToClipboard(url string) error {
	selected, err := selectClipboardCommand(exec.LookPath)
	if err != nil {
		return err
	}
	cmd := exec.Command(selected[0], selected[1:]...)
	cmd.Stdin = bytes.NewBufferString(url)
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

func browserCommand(goos, rawURL string) (string, []string) {
	switch goos {
	case "darwin":
		return "open", []string{rawURL}
	case "windows":
		return "rundll32", []string{"url.dll,FileProtocolHandler", rawURL}
	default:
		return "xdg-open", []string{rawURL}
	}
}

func selectClipboardCommand(lookPath func(string) (string, error)) ([]string, error) {
	commands := [][]string{
		{"pbcopy"},
		{"xclip", "-selection", "clipboard"},
		{"wl-copy"},
	}
	for _, c := range commands {
		if _, err := lookPath(c[0]); err == nil {
			return c, nil
		}
	}
	return nil, fmt.Errorf("no clipboard command available")
}
