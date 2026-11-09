package addon

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// UsageInfo is the traffic state read from a Subscription-Userinfo header.
// Total 0 means unlimited.
type UsageInfo struct {
	Used, Total float64
}

// parseUserinfo parses "upload=1; download=2; total=3; expire=4". Used is
// upload+download. It fails when none of upload, download and total is present.
func parseUserinfo(h string) (UsageInfo, error) {
	var info UsageInfo
	found := false
	for _, part := range strings.Split(h, ";") {
		k, v, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok {
			continue
		}
		n, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
		if err != nil {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(k)) {
		case "upload", "download":
			info.Used += n
			found = true
		case "total":
			info.Total = n
			found = true
		}
	}
	if !found {
		return UsageInfo{}, fmt.Errorf("no traffic fields in %q", h)
	}
	return info, nil
}

type usageEntry struct {
	mu   sync.Mutex
	info UsageInfo
	at   time.Time
	ok   bool
	// failedAt is the last failed fetch; retries wait retryAfter so a dead
	// provider is not hit (and waited on) by every subscription request.
	failedAt time.Time
}

const retryAfter = 30 * time.Second

// usageCache fetches and caches usage per key (one key per add-on).
type usageCache struct {
	mu      sync.Mutex
	entries map[string]*usageEntry
	http    *http.Client
}

func newUsageCache() *usageCache {
	return &usageCache{entries: map[string]*usageEntry{}, http: &http.Client{Timeout: 10 * time.Second}}
}

// get returns the cached usage when fresh, otherwise refetches it. If the
// refetch fails the last good value is returned; ok is false only when no
// fetch has ever succeeded for key.
func (c *usageCache) get(ctx context.Context, key string, u Usage) (UsageInfo, bool) {
	c.mu.Lock()
	e := c.entries[key]
	if e == nil {
		e = &usageEntry{}
		c.entries[key] = e
	}
	c.mu.Unlock()

	e.mu.Lock()
	defer e.mu.Unlock()
	if e.ok && time.Since(e.at) < u.Cache {
		return e.info, true
	}
	if !e.failedAt.IsZero() && time.Since(e.failedAt) < min(retryAfter, u.Cache) {
		return e.info, e.ok
	}
	if info, err := c.fetch(ctx, u); err == nil {
		e.info, e.at, e.ok, e.failedAt = info, time.Now(), true, time.Time{}
	} else {
		e.failedAt = time.Now()
	}
	return e.info, e.ok
}

func (c *usageCache) fetch(ctx context.Context, u Usage) (UsageInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.URL, nil)
	if err != nil {
		return UsageInfo{}, err
	}
	for k, v := range u.Headers {
		req.Header.Set(k, v)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return UsageInfo{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return UsageInfo{}, fmt.Errorf("usage status %d", resp.StatusCode)
	}
	h := resp.Header.Get("Subscription-Userinfo")
	if h == "" {
		return UsageInfo{}, errors.New("no Subscription-Userinfo header")
	}
	return parseUserinfo(h)
}
