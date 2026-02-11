package view

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

const inlineImagePreviewRows = 18

func RenderInlineImagePreview(imageURL string, width int) (string, error) {
	if width < 30 {
		width = 40
	}

	chafaPath, err := exec.LookPath("chafa")
	if err != nil {
		return "", fmt.Errorf("chafa is not installed")
	}

	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Get(imageURL)
	if err != nil {
		return "", fmt.Errorf("download image: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("download image: status %d", resp.StatusCode)
	}

	imageData, err := io.ReadAll(io.LimitReader(resp.Body, 5*1024*1024))
	if err != nil {
		return "", fmt.Errorf("read image: %w", err)
	}

	args := []string{
		"--size", fmt.Sprintf("%dx%d", width, inlineImagePreviewRows),
		"--view-size", fmt.Sprintf("%dx%d", width, inlineImagePreviewRows),
		"--align", "top,center",
		"--format", "symbols",
		"-",
	}
	if SupportsKittyGraphics() {
		args = []string{
			"--size", fmt.Sprintf("%dx%d", width, inlineImagePreviewRows),
			"--view-size", fmt.Sprintf("%dx%d", width, inlineImagePreviewRows),
			"--align", "top,center",
			"--format", "kitty",
			"--passthrough", KittyPassthroughMode(),
			"--relative", "on",
			"-",
		}
	}
	cmd := exec.Command(chafaPath, args...)
	cmd.Stdin = bytes.NewReader(imageData)
	output, err := cmd.CombinedOutput()
	raw := string(output)
	trimmed := strings.TrimSpace(raw)

	if err != nil {
		return "", fmt.Errorf("render image via chafa: %w: %s", err, trimmed)
	}
	if SupportsKittyGraphics() && ContainsKittyGraphicsEscape(raw) {
		return strings.TrimRight(raw, "\r\n"), nil
	}
	if trimmed == "" {
		return "", fmt.Errorf("empty output")
	}
	return trimmed, nil
}

func SupportsKittyGraphics() bool {
	if os.Getenv("KITTY_WINDOW_ID") != "" {
		return true
	}
	termProgram := strings.ToLower(strings.TrimSpace(os.Getenv("TERM_PROGRAM")))
	if strings.Contains(termProgram, "ghostty") || strings.Contains(termProgram, "kitty") {
		return true
	}
	term := strings.ToLower(strings.TrimSpace(os.Getenv("TERM")))
	return strings.Contains(term, "xterm-kitty") || strings.Contains(term, "ghostty")
}

func ContainsKittyGraphicsEscape(s string) bool {
	return strings.Contains(s, "\x1b_G")
}

func KittyRenderedLineCount(s string) int {
	if strings.TrimSpace(s) == "" {
		return 0
	}
	return strings.Count(s, "\n") + 1
}

func ClearKittyGraphicsSequence() string {
	base := "\x1b_Ga=d,d=A\x1b\\"
	if os.Getenv("TMUX") == "" {
		return base
	}
	escaped := strings.ReplaceAll(base, "\x1b", "\x1b\x1b")
	return "\x1bPtmux;\x1b" + escaped + "\x1b\\"
}

func KittyPassthroughMode() string {
	if os.Getenv("TMUX") != "" {
		return "screen"
	}
	return "none"
}
