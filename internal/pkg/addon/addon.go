// Package addon appends the configs of extra Remnawave users to the main
// user's subscription. An add-on user is named after the main one with a
// fixed prefix and/or suffix (e.g. "premium_<username>"); billing creates it
// for a paid option with its own squad, traffic limit and reset strategy,
// and the panel itself flips it to LIMITED when the limit is spent.
//
// Add-ons are described in a YAML file (see addons.example.yml).
package addon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"

	"subpage/internal/pkg/subpage"
)

// Endpoint paths from @remnawave/backend-contract@3.4.15.
const (
	pathUserByShortUUID = "/api/users/by-short-uuid/%s" // GetUserByShortUuidCommand
	pathUserByUsername  = "/api/users/by-username/%s"   // GetUserByUsernameCommand
	pathSubscription    = "/api/sub/%s"                 // public subscription controller
)

const StatusActive = "ACTIVE"

const defaultTimeout = 2 * time.Second

// Addon is one entry of the add-ons file.
type Addon struct {
	// Name is only used in logs; defaults to Prefix+Suffix.
	Name   string `yaml:"name"`
	Prefix string `yaml:"prefix"`
	Suffix string `yaml:"suffix"`
	// Remark rewrites each add-on config's name. Placeholders: {remark}
	// (original name), {remaining}, {limit}, {used} (GB, one decimal).
	Remark string `yaml:"remark"`
	// RemarkUnlimited is used when trafficLimitBytes is 0; defaults to Remark.
	RemarkUnlimited string `yaml:"remarkUnlimited"`
	// Stubs maps a non-ACTIVE user status (LIMITED, EXPIRED, DISABLED) to
	// the name of a single unusable placeholder config. A status that is
	// missing or empty adds nothing.
	Stubs map[string]string `yaml:"stubs"`
}

func (a Addon) username(main string) string { return a.Prefix + main + a.Suffix }

// Config is the whole add-ons file.
type Config struct {
	// Timeout bounds all add-on lookups of one request; on timeout the main
	// subscription is served alone.
	Timeout time.Duration `yaml:"timeout"`
	Addons  []Addon       `yaml:"addons"`
}

// LoadConfig reads the add-ons file. A missing file means no add-ons; so
// does a directory, which is what Docker creates for a bind mount whose
// source file doesn't exist.
func LoadConfig(path string) (Config, error) {
	cfg := Config{Timeout: defaultTimeout}
	if fi, err := os.Stat(path); err == nil && fi.IsDir() {
		return cfg, nil
	}
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		return cfg, err
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = defaultTimeout
	}
	for i := range cfg.Addons {
		a := &cfg.Addons[i]
		if a.Prefix == "" && a.Suffix == "" {
			return cfg, fmt.Errorf("addons[%d]: prefix or suffix is required", i)
		}
		if a.Name == "" {
			a.Name = a.Prefix + a.Suffix
		}
		if a.Remark == "" {
			a.Remark = "{remark}"
		}
		if a.RemarkUnlimited == "" {
			a.RemarkUnlimited = a.Remark
		}
		stubs := make(map[string]string, len(a.Stubs))
		for status, text := range a.Stubs {
			stubs[strings.ToUpper(status)] = text
		}
		a.Stubs = stubs
	}
	return cfg, nil
}

type Service struct {
	panel *subpage.PanelClient
	cfg   Config
}

func NewService(panel *subpage.PanelClient, cfg Config) *Service {
	return &Service{panel: panel, cfg: cfg}
}

func (s *Service) Enabled() bool { return s != nil && len(s.cfg.Addons) > 0 }

func (s *Service) Timeout() time.Duration { return s.cfg.Timeout }

// panelUser is the subset of GetUserBy*Command.ResponseSchema we need.
type panelUser struct {
	ShortUUID         string  `json:"shortUuid"`
	Username          string  `json:"username"`
	Status            string  `json:"status"`
	TrafficLimitBytes float64 `json:"trafficLimitBytes"`
	UsedTrafficBytes  float64 `json:"usedTrafficBytes"` // pre-2.x layout
	UserTraffic       *struct {
		UsedTrafficBytes float64 `json:"usedTrafficBytes"`
	} `json:"userTraffic"`
}

func (u *panelUser) used() float64 {
	if u.UserTraffic != nil {
		return u.UserTraffic.UsedTrafficBytes
	}
	return u.UsedTrafficBytes
}

// errNotFound means the panel answered 404: the user simply doesn't exist.
var errNotFound = errors.New("not found")

func (s *Service) getUser(ctx context.Context, path, clientIP string) (*panelUser, error) {
	resp, err := s.panel.Get(ctx, path, clientIP, nil)
	if err != nil {
		return nil, err
	}
	if resp.Status == http.StatusNotFound {
		return nil, errNotFound
	}
	if !resp.OK {
		return nil, fmt.Errorf("panel status %d", resp.Status)
	}
	var wrapper struct {
		Response panelUser `json:"response"`
	}
	if err := json.Unmarshal(resp.Body, &wrapper); err != nil {
		return nil, err
	}
	return &wrapper.Response, nil
}

// Result is what one add-on lookup found; Addition is nil when nothing is
// appended.
type Result struct {
	Addon    string
	Found    bool
	Status   string
	Addition *Addition
	Err      error
}

// Resolve looks up every configured add-on for the subscription identified
// by mainShortUUID and returns the main username plus one Result per
// add-on, in config order. Add-on usernames are built only from the
// username the panel returns for mainShortUUID, never from request input.
// forward are the client's request headers, sent to /api/sub so the panel
// picks the same output format as for the main user.
func (s *Service) Resolve(ctx context.Context, mainShortUUID, clientType, clientIP string, forward http.Header) (string, []Result, error) {
	main, err := s.getUser(ctx, fmt.Sprintf(pathUserByShortUUID, url.PathEscape(mainShortUUID)), clientIP)
	if errors.Is(err, errNotFound) {
		return "", nil, nil
	}
	if err != nil {
		return "", nil, fmt.Errorf("main user lookup: %w", err)
	}

	results := make([]Result, len(s.cfg.Addons))
	var wg sync.WaitGroup
	for i, a := range s.cfg.Addons {
		wg.Go(func() {
			results[i] = s.resolveOne(ctx, a, main.Username, clientType, clientIP, forward.Clone())
		})
	}
	wg.Wait()
	return main.Username, results, nil
}

func (s *Service) resolveOne(ctx context.Context, a Addon, mainUsername, clientType, clientIP string, forward http.Header) Result {
	res := Result{Addon: a.Name}
	user, err := s.getUser(ctx, fmt.Sprintf(pathUserByUsername, url.PathEscape(a.username(mainUsername))), clientIP)
	if errors.Is(err, errNotFound) {
		return res
	}
	if err != nil {
		res.Err = fmt.Errorf("user lookup: %w", err)
		return res
	}
	res.Found = true
	res.Status = user.Status

	if user.Status != StatusActive {
		if stub := a.Stubs[user.Status]; stub != "" {
			res.Addition = &Addition{StubRemark: stub}
		}
		return res
	}

	path := fmt.Sprintf(pathSubscription, url.PathEscape(user.ShortUUID))
	if clientType != "" {
		path += "/" + clientType
	}
	resp, err := s.panel.Get(ctx, path, clientIP, forward)
	if err != nil {
		res.Err = fmt.Errorf("subscription: %w", err)
		return res
	}
	if !resp.OK {
		res.Err = fmt.Errorf("subscription: panel status %d", resp.Status)
		return res
	}
	res.Addition = &Addition{Body: resp.Body, Rename: renamer(a, user)}
	return res
}

func renamer(a Addon, u *panelUser) func(string) string {
	used, limit := u.used(), u.TrafficLimitBytes
	return func(remark string) string {
		if limit == 0 {
			return strings.NewReplacer("{remark}", remark, "{used}", FormatGB(used)).Replace(a.RemarkUnlimited)
		}
		return strings.NewReplacer(
			"{remark}", remark,
			"{remaining}", FormatGB(math.Max(0, limit-used)),
			"{limit}", FormatGB(limit),
			"{used}", FormatGB(used),
		).Replace(a.Remark)
	}
}

// FormatGB formats bytes as GB (1024^3, as the panel does) with one
// decimal, dropping a trailing ".0": 10 GiB -> "10", 6.6 GiB -> "6.6".
func FormatGB(bytes float64) string {
	gb := math.Round(bytes/(1<<30)*10) / 10
	return strconv.FormatFloat(gb, 'f', -1, 64)
}
