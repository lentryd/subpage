package subpage

import (
	"crypto/rand"
	"strings"
)

const nanoidAlphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789_-"

// NewSessionID returns a random 32-char id, analogous to nanoid(32).
func NewSessionID() string {
	buf := make([]byte, 32)
	_, _ = rand.Read(buf) // crypto/rand.Read only fails on catastrophic system error
	b := make([]byte, 32)
	for i, v := range buf {
		b[i] = nanoidAlphabet[int(v)%len(nanoidAlphabet)]
	}
	return string(b)
}

// IsGenericPath mirrors isGenericPath: static-asset-like paths that should
// never reach the subscription-serving logic.
func IsGenericPath(path string) bool {
	for _, suffix := range []string{"favicon.ico", "robots.txt", ".png", ".jpg", ".jpeg", ".gif", ".svg", ".webp", ".ico"} {
		if strings.Contains(path, suffix) {
			return true
		}
	}
	return false
}

// IsBrowser mirrors isBrowser: a coarse user-agent sniff to decide whether
// to render the HTML page vs. return a raw subscription payload.
func IsBrowser(userAgent string) bool {
	for _, marker := range []string{"Mozilla", "Chrome", "Safari", "Firefox", "Opera", "Edge", "TelegramBot", "WhatsApp"} {
		if strings.Contains(userAgent, marker) {
			return true
		}
	}
	return false
}
