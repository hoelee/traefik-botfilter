package traefik_botfilter

import (
	"fmt"
	"net/netip"
	"net/textproto"
	"strings"
	"time"
)

// Config is the plugin configuration exposed by Traefik's dynamic file
// provider. All durations are expressed as integers because Traefik plugin
// configuration is deliberately kept YAML-friendly.
type Config struct {
	StatusCode                  int      `json:"statusCode,omitempty" yaml:"statusCode,omitempty" toml:"statusCode,omitempty"`
	RequireUserAgent            bool     `json:"requireUserAgent,omitempty" yaml:"requireUserAgent,omitempty" toml:"requireUserAgent,omitempty"`
	RequireAccept               bool     `json:"requireAccept,omitempty" yaml:"requireAccept,omitempty" toml:"requireAccept,omitempty"`
	RequireHost                 bool     `json:"requireHost,omitempty" yaml:"requireHost,omitempty" toml:"requireHost,omitempty"`
	BrowserValidation           bool     `json:"browserValidation,omitempty" yaml:"browserValidation,omitempty" toml:"browserValidation,omitempty"`
	WhitelistCIDRs              []string `json:"whitelistCIDRs,omitempty" yaml:"whitelistCIDRs,omitempty" toml:"whitelistCIDRs,omitempty"`
	TemporaryBanMinutes         int      `json:"temporaryBanMinutes,omitempty" yaml:"temporaryBanMinutes,omitempty" toml:"temporaryBanMinutes,omitempty"`
	BlockedUserAgents           []string `json:"blockedUserAgents,omitempty" yaml:"blockedUserAgents,omitempty" toml:"blockedUserAgents,omitempty"`
	BlockedPaths                []string `json:"blockedPaths,omitempty" yaml:"blockedPaths,omitempty" toml:"blockedPaths,omitempty"`
	BlockedExtensions           []string `json:"blockedExtensions,omitempty" yaml:"blockedExtensions,omitempty" toml:"blockedExtensions,omitempty"`
	ScoreThreshold              int      `json:"scoreThreshold,omitempty" yaml:"scoreThreshold,omitempty" toml:"scoreThreshold,omitempty"`
	ScoreWindowMinutes          int      `json:"scoreWindowMinutes,omitempty" yaml:"scoreWindowMinutes,omitempty" toml:"scoreWindowMinutes,omitempty"`
	MaxTrackedIPs               int      `json:"maxTrackedIPs,omitempty" yaml:"maxTrackedIPs,omitempty" toml:"maxTrackedIPs,omitempty"`
	MaxScoreEventsPerIP         int      `json:"maxScoreEventsPerIP,omitempty" yaml:"maxScoreEventsPerIP,omitempty" toml:"maxScoreEventsPerIP,omitempty"`
	EmptyUserAgentScore         int      `json:"emptyUserAgentScore,omitempty" yaml:"emptyUserAgentScore,omitempty" toml:"emptyUserAgentScore,omitempty"`
	MissingAcceptScore          int      `json:"missingAcceptScore,omitempty" yaml:"missingAcceptScore,omitempty" toml:"missingAcceptScore,omitempty"`
	BlockedUserAgentScore       int      `json:"blockedUserAgentScore,omitempty" yaml:"blockedUserAgentScore,omitempty" toml:"blockedUserAgentScore,omitempty"`
	BadPathScore                int      `json:"badPathScore,omitempty" yaml:"badPathScore,omitempty" toml:"badPathScore,omitempty"`
	RandomArticleScore          int      `json:"randomArticleScore,omitempty" yaml:"randomArticleScore,omitempty" toml:"randomArticleScore,omitempty"`
	NotFoundScore               int      `json:"notFoundScore,omitempty" yaml:"notFoundScore,omitempty" toml:"notFoundScore,omitempty"`
	FakeBrowserScore            int      `json:"fakeBrowserScore,omitempty" yaml:"fakeBrowserScore,omitempty" toml:"fakeBrowserScore,omitempty"`
	RandomArticlePatterns       []string `json:"randomArticlePatterns,omitempty" yaml:"randomArticlePatterns,omitempty" toml:"randomArticlePatterns,omitempty"`
	ClientIPHeader              string   `json:"clientIPHeader,omitempty" yaml:"clientIPHeader,omitempty" toml:"clientIPHeader,omitempty"`
	TrustedProxyCIDRs           []string `json:"trustedProxyCIDRs,omitempty" yaml:"trustedProxyCIDRs,omitempty" toml:"trustedProxyCIDRs,omitempty"`
	LogBlockedRequests          bool     `json:"logBlockedRequests,omitempty" yaml:"logBlockedRequests,omitempty" toml:"logBlockedRequests,omitempty"`
}

// CreateConfig creates the default configuration. The defaults protect common
// public HTTP services without requiring a third-party dependency.
func CreateConfig() *Config {
	return &Config{
		StatusCode:            403,
		TemporaryBanMinutes:   15,
		ScoreThreshold:        100,
		ScoreWindowMinutes:    15,
		MaxTrackedIPs:         50000,
		MaxScoreEventsPerIP:   16,
		EmptyUserAgentScore:   40,
		MissingAcceptScore:    20,
		BlockedUserAgentScore: 80,
		BadPathScore:          50,
		RandomArticleScore:    15,
		NotFoundScore:         40,
		FakeBrowserScore:      40,
		RandomArticlePatterns: []string{"/content/"},
	}
}

type compiledConfig struct {
	Config
	banDuration    time.Duration
	scoreWindow    time.Duration
	whitelist      []netip.Prefix
	trustedProxies []netip.Prefix
	blockedAgents  []string
	blockedPaths   []string
	blockedExts    []string
	randomPaths    []string
	clientIPHeader string
}

func compileConfig(input *Config) (*compiledConfig, error) {
	if input == nil {
		return nil, fmt.Errorf("botfilter: configuration is nil")
	}

	// Copy scalar fields and slices so a later configuration reload cannot
	// mutate an already-running middleware instance.
	cfg := *input
	cfg.WhitelistCIDRs = append([]string(nil), input.WhitelistCIDRs...)
	cfg.TrustedProxyCIDRs = append([]string(nil), input.TrustedProxyCIDRs...)
	cfg.BlockedUserAgents = append([]string(nil), input.BlockedUserAgents...)
	cfg.BlockedPaths = append([]string(nil), input.BlockedPaths...)
	cfg.BlockedExtensions = append([]string(nil), input.BlockedExtensions...)
	cfg.RandomArticlePatterns = append([]string(nil), input.RandomArticlePatterns...)

	defaults := CreateConfig()
	applyDefaults(&cfg, defaults)

	if cfg.StatusCode < 400 || cfg.StatusCode > 599 {
		return nil, fmt.Errorf("botfilter: statusCode must be between 400 and 599")
	}
	if cfg.TemporaryBanMinutes <= 0 {
		return nil, fmt.Errorf("botfilter: temporaryBanMinutes must be greater than zero")
	}
	if cfg.ScoreThreshold <= 0 || cfg.ScoreWindowMinutes <= 0 {
		return nil, fmt.Errorf("botfilter: scoreThreshold and scoreWindowMinutes must be greater than zero")
	}
	if cfg.MaxTrackedIPs <= 0 || cfg.MaxScoreEventsPerIP <= 0 {
		return nil, fmt.Errorf("botfilter: maxTrackedIPs and maxScoreEventsPerIP must be greater than zero")
	}

	whitelist, err := parseCIDRs(cfg.WhitelistCIDRs, "whitelistCIDRs")
	if err != nil {
		return nil, err
	}
	trusted, err := parseCIDRs(cfg.TrustedProxyCIDRs, "trustedProxyCIDRs")
	if err != nil {
		return nil, err
	}

	return &compiledConfig{
		Config:         cfg,
		banDuration:    time.Duration(cfg.TemporaryBanMinutes) * time.Minute,
		scoreWindow:    time.Duration(cfg.ScoreWindowMinutes) * time.Minute,
		whitelist:      whitelist,
		trustedProxies: trusted,
		blockedAgents:  normaliseTokens(cfg.BlockedUserAgents),
		blockedPaths:   normalisePaths(cfg.BlockedPaths),
		blockedExts:    normaliseExtensions(cfg.BlockedExtensions),
		randomPaths:    normalisePaths(cfg.RandomArticlePatterns),
		clientIPHeader: textproto.CanonicalMIMEHeaderKey(strings.TrimSpace(cfg.ClientIPHeader)),
	}, nil
}

func applyDefaults(cfg, defaults *Config) {
	if cfg.StatusCode == 0 {
		cfg.StatusCode = defaults.StatusCode
	}
	if cfg.TemporaryBanMinutes == 0 {
		cfg.TemporaryBanMinutes = defaults.TemporaryBanMinutes
	}
	if cfg.ScoreThreshold == 0 {
		cfg.ScoreThreshold = defaults.ScoreThreshold
	}
	if cfg.ScoreWindowMinutes == 0 {
		cfg.ScoreWindowMinutes = defaults.ScoreWindowMinutes
	}
	if cfg.MaxTrackedIPs == 0 {
		cfg.MaxTrackedIPs = defaults.MaxTrackedIPs
	}
	if cfg.MaxScoreEventsPerIP == 0 {
		cfg.MaxScoreEventsPerIP = defaults.MaxScoreEventsPerIP
	}
	// Score fields deliberately do not receive fallback values here. Traefik
	// starts from CreateConfig(), so omitted values retain their defaults, while
	// an explicit YAML zero remains a useful way to disable one signal.
}

func parseCIDRs(values []string, field string) ([]netip.Prefix, error) {
	result := make([]netip.Prefix, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		prefix, err := netip.ParsePrefix(value)
		if err != nil {
			return nil, fmt.Errorf("botfilter: invalid %s entry %q: %w", field, value, err)
		}
		result = append(result, prefix.Masked())
	}
	return result, nil
}
