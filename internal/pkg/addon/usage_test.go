package addon

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestParseUserinfo(t *testing.T) {
	got, err := parseUserinfo("upload=1; download=2;total=10; expire=99")
	if err != nil || got != (UsageInfo{Used: 3, Total: 10}) {
		t.Fatalf("%+v %v", got, err)
	}
	got, err = parseUserinfo("total=0; download=5")
	if err != nil || got != (UsageInfo{Used: 5, Total: 0}) {
		t.Fatalf("%+v %v", got, err)
	}
	for _, bad := range []string{"", "garbage", "expire=1"} {
		if _, err := parseUserinfo(bad); err == nil {
			t.Errorf("%q: expected error", bad)
		}
	}
}

func usageServer(t *testing.T, hits *atomic.Int32, status *atomic.Int32) (*httptest.Server, *http.Header) {
	t.Helper()
	var seen http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		seen = r.Header.Clone()
		if s := int(status.Load()); s != 0 && s != 200 {
			w.WriteHeader(s)
			return
		}
		w.Header().Set("Subscription-Userinfo", "upload=1073741824; download=2147483648; total=10737418240")
		_, _ = w.Write([]byte("ok"))
	}))
	t.Cleanup(srv.Close)
	return srv, &seen
}

func TestUsageCacheSendsHeaders(t *testing.T) {
	var hits, status atomic.Int32
	srv, seen := usageServer(t, &hits, &status)
	c := newUsageCache()
	u := Usage{URL: srv.URL, Headers: map[string]string{"X-Hwid": "dev-1", "User-Agent": "Happ/1.0"}, Cache: time.Minute}
	got, ok := c.get(context.Background(), "k", u)
	if !ok || got != (UsageInfo{Used: 3 << 30, Total: 10 << 30}) {
		t.Fatalf("%+v %v", got, ok)
	}
	if seen.Get("X-Hwid") != "dev-1" || seen.Get("User-Agent") != "Happ/1.0" {
		t.Fatalf("headers: %v", *seen)
	}
}

func TestUsageCacheTTL(t *testing.T) {
	var hits, status atomic.Int32
	srv, _ := usageServer(t, &hits, &status)
	c := newUsageCache()
	u := Usage{URL: srv.URL, Cache: 50 * time.Millisecond}
	c.get(context.Background(), "k", u)
	c.get(context.Background(), "k", u)
	if hits.Load() != 1 {
		t.Fatalf("hits = %d, want 1 within TTL", hits.Load())
	}
	time.Sleep(80 * time.Millisecond)
	c.get(context.Background(), "k", u)
	if hits.Load() != 2 {
		t.Fatalf("hits = %d, want 2 after TTL", hits.Load())
	}
}

func TestUsageCacheStaleOnError(t *testing.T) {
	var hits, status atomic.Int32
	srv, _ := usageServer(t, &hits, &status)
	c := newUsageCache()
	u := Usage{URL: srv.URL, Cache: 10 * time.Millisecond}
	first, _ := c.get(context.Background(), "k", u)
	status.Store(500)
	time.Sleep(30 * time.Millisecond)
	got, ok := c.get(context.Background(), "k", u)
	if !ok || got != first {
		t.Fatalf("stale: %+v %v want %+v", got, ok, first)
	}
}

func TestUsageCacheNeverFetched(t *testing.T) {
	var hits, status atomic.Int32
	status.Store(500)
	srv, _ := usageServer(t, &hits, &status)
	c := newUsageCache()
	if _, ok := c.get(context.Background(), "k", Usage{URL: srv.URL, Cache: time.Minute}); ok {
		t.Fatal("expected ok=false")
	}
}

func TestUsageCacheBacksOffAfterFailure(t *testing.T) {
	var hits, status atomic.Int32
	status.Store(500)
	srv, _ := usageServer(t, &hits, &status)
	c := newUsageCache()
	u := Usage{URL: srv.URL, Cache: time.Minute}
	for range 3 {
		c.get(context.Background(), "k", u)
	}
	if hits.Load() != 1 {
		t.Fatalf("hits = %d, want 1: a failing provider must not be hit on every request", hits.Load())
	}
}
