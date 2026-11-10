package addon

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
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

// maxUsageBody caps how much of the usage subscription is read for `match`.
const maxUsageBody = 4 << 20

// usageData is one fetch of the usage URL: traffic numbers from the
// Subscription-Userinfo header (hasInfo false when absent) and, when asked
// for, the configs of the response body.
type usageData struct {
	info    UsageInfo
	hasInfo bool
	links   []providerLink
}

type usageEntry struct {
	mu   sync.Mutex
	data usageData
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

// get returns the cached usage numbers when fresh, otherwise refetches them.
// If the refetch fails the last good value is returned; ok is false when no
// fetch has ever succeeded for key or the response carried no numbers.
func (c *usageCache) get(ctx context.Context, key string, u Usage) (UsageInfo, bool) {
	d, ok := c.load(ctx, key, u, false)
	return d.info, ok && d.hasInfo
}

// load is get plus, when withLinks is set, the configs found in the body of
// the response (and then a missing Subscription-Userinfo header is no error).
func (c *usageCache) load(ctx context.Context, key string, u Usage, withLinks bool) (usageData, bool) {
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
		return e.data, true
	}
	if !e.failedAt.IsZero() && time.Since(e.failedAt) < min(retryAfter, u.Cache) {
		return e.data, e.ok
	}
	if d, err := c.fetch(ctx, u, withLinks); err == nil {
		e.data, e.at, e.ok, e.failedAt = d, time.Now(), true, time.Time{}
	} else {
		slog.Warn("add-on usage fetch failed", "url", u.URL, "err", err)
		e.failedAt = time.Now()
	}
	return e.data, e.ok
}

func (c *usageCache) fetch(ctx context.Context, u Usage, withLinks bool) (usageData, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.URL, nil)
	if err != nil {
		return usageData{}, err
	}
	for k, v := range u.Headers {
		req.Header.Set(k, v)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return usageData{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return usageData{}, fmt.Errorf("usage status %d", resp.StatusCode)
	}
	var d usageData
	if h := resp.Header.Get("Subscription-Userinfo"); h != "" {
		if d.info, err = parseUserinfo(h); err != nil {
			if !withLinks {
				return usageData{}, err
			}
		} else {
			d.hasInfo = true
		}
	} else if !withLinks {
		return usageData{}, errors.New("no Subscription-Userinfo header")
	}
	if withLinks {
		body, err := io.ReadAll(io.LimitReader(resp.Body, maxUsageBody))
		if err != nil {
			return usageData{}, err
		}
		d.links = parseProviderLinks(body)
	}
	return d, nil
}
