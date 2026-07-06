package subpage

import (
	"encoding/json"
	"log/slog"
	"os"
	"sync"
)

// SubpageDefaultConfigUUID mirrors SUBPAGE_DEFAULT_CONFIG_UUID from
// @remnawave/subscription-page-types.
const SubpageDefaultConfigUUID = "00000000-0000-0000-0000-000000000000"

// BaseSettings is the subset of a raw subpage config needed to render the
// HTML shell, mirroring subpage-config.service.ts getBaseSettings.
type BaseSettings struct {
	MetaTitle          string `json:"metaTitle"`
	MetaDescription    string `json:"metaDescription"`
	ShowConnectionKeys bool   `json:"showConnectionKeys"`
	HideGetLinkButton  bool   `json:"hideGetLinkButton"`
}

var defaultBaseSettings = BaseSettings{
	MetaTitle:          "Subscription Page",
	MetaDescription:    "Subscription Page",
	ShowConnectionKeys: false,
	HideGetLinkButton:  false,
}

// rawConfig is an opaque subpage config: the app-config route serves Raw
// (the config object itself, not the panel's {"response":{"config":...}}
// envelope) to the client verbatim, since the client validates it against
// SubscriptionPageRawConfigSchema. The Go side only needs baseSettings out
// of it, not every field.
type rawConfig struct {
	BaseSettings BaseSettings    `json:"baseSettings"`
	Raw          json.RawMessage `json:"-"`
}

// ConfigStore caches subpage configs fetched from the Remnawave panel at
// startup, keyed by subpageConfigUuid. Mirrors SubpageConfigService.
type ConfigStore struct {
	mu                sync.RWMutex
	configs           map[string]rawConfig
	internalJWTSecret string
	staticConfigUUID  string
	panel             *PanelClient
}

func NewConfigStore(panel *PanelClient, internalJWTSecret, staticConfigUUID string) *ConfigStore {
	return &ConfigStore{
		configs:           make(map[string]rawConfig),
		internalJWTSecret: internalJWTSecret,
		staticConfigUUID:  staticConfigUUID,
		panel:             panel,
	}
}

// Bootstrap loads every subpage config from the panel into memory. Exits
// the process on failure, matching onApplicationBootstrap's fatal-and-exit
// behavior in the Nest original.
func (s *ConfigStore) Bootstrap() {
	listResp, err := s.panel.GetSubscriptionPageConfigList()
	if err != nil || !listResp.OK {
		slog.Error("failed to fetch subscription page config list", "err", err)
		os.Exit(1)
	}

	var list struct {
		Response struct {
			Configs []struct {
				UUID string `json:"uuid"`
			} `json:"configs"`
		} `json:"response"`
	}
	if err := json.Unmarshal(listResp.Body, &list); err != nil {
		slog.Error("failed to parse subscription page config list", "err", err)
		os.Exit(1)
	}
	if len(list.Response.Configs) == 0 {
		slog.Error("no subscription page configs returned by panel")
		os.Exit(1)
	}

	loaded := make(map[string]rawConfig, len(list.Response.Configs))
	for _, c := range list.Response.Configs {
		resp, err := s.panel.GetSubscriptionPageConfigByUUID(c.UUID)
		if err != nil || !resp.OK {
			slog.Error("failed to fetch subscription page config", "uuid", c.UUID, "err", err)
			os.Exit(1)
		}
		var wrapper struct {
			Response struct {
				Config json.RawMessage `json:"config"`
			} `json:"response"`
		}
		if err := json.Unmarshal(resp.Body, &wrapper); err != nil {
			slog.Error("invalid subscription page config", "uuid", c.UUID, "err", err)
			os.Exit(1)
		}
		var cfg rawConfig
		if err := json.Unmarshal(wrapper.Response.Config, &cfg); err != nil {
			slog.Error("invalid subscription page config", "uuid", c.UUID, "err", err)
			os.Exit(1)
		}
		cfg.Raw = wrapper.Response.Config
		loaded[c.UUID] = cfg
		slog.Info("loaded subscription page config", "uuid", c.UUID)
	}

	if len(loaded) == 0 {
		slog.Error("subscription page config map ended up empty")
		os.Exit(1)
	}

	s.mu.Lock()
	s.configs = loaded
	s.mu.Unlock()
	slog.Info("subscription page configs loaded", "count", len(loaded))
}

// finalUUID mirrors getFinalSubpageConfigUuid: in default/multi-tenant mode
// (server's static uuid is the well-known default) prefer the per-request
// uuid coming from the panel; otherwise always use the static uuid.
func (s *ConfigStore) finalUUID(perRequestUUID string) string {
	if s.staticConfigUUID == SubpageDefaultConfigUUID && perRequestUUID != "" {
		return perRequestUUID
	}
	return s.staticConfigUUID
}

// GetBaseSettings returns render-time settings for a config uuid coming
// from the panel (subpageConfig.subpageConfigUuid), falling back to
// sensible defaults if not found. Mirrors getBaseSettings.
func (s *ConfigStore) GetBaseSettings(perRequestUUID string) BaseSettings {
	uuid := s.finalUUID(perRequestUUID)
	s.mu.RLock()
	defer s.mu.RUnlock()
	if cfg, ok := s.configs[uuid]; ok {
		return cfg.BaseSettings
	}
	return defaultBaseSettings
}

// EncryptedUUID returns the encrypted `su` claim value for a given
// per-request panel uuid, mirroring getEncryptedSubpageConfigUuid.
func (s *ConfigStore) EncryptedUUID(perRequestUUID string) (string, error) {
	return EncryptUUID(s.finalUUID(perRequestUUID), s.internalJWTSecret)
}

// RawConfigJSON returns the raw config JSON for the given encrypted `su`
// claim value (as decrypted), for serving the app-config route. Mirrors
// getSubscriptionPageConfig.
func (s *ConfigStore) RawConfigJSON(encryptedUUID string) (json.RawMessage, bool) {
	uuid, ok := DecryptUUID(encryptedUUID, s.internalJWTSecret)
	if !ok {
		return nil, false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	cfg, ok := s.configs[uuid]
	if !ok {
		return nil, false
	}
	return cfg.Raw, true
}
