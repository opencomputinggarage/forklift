// Package repoconfig defines per-repository configuration: caching policy, age
// policy, and package approval policy. It is stored as JSON in
// repositories.config_json and shared by the admin API and the proxy cache core.
package repoconfig

import (
	"encoding/json"
	"fmt"
	"net/netip"
	"path"
	"strconv"
	"strings"
	"time"
)

// Config is the per-repository config payload.
type Config struct {
	Cache        CacheConfig              `json:"cache"`
	AgePolicy    AgePolicyConfig          `json:"age_policy"`
	Approval     ApprovalConfig           `json:"approval"`
	Retention    RetentionConfig          `json:"retention"`
	Vuln         VulnPolicyConfig         `json:"vuln"`
	License      LicensePolicyConfig      `json:"license"`
	ArtifactScan ArtifactScanPolicyConfig `json:"artifact_scan"`
	Group        GroupConfig              `json:"group"`
	IPACL        IPACLConfig              `json:"ip_acl"`
	Notify       NotifyConfig             `json:"notify"`
}

// NotifyConfig selects which notification receivers (named alarm channels
// managed in the admin console) are alerted for this repository's events —
// currently a package entering the approval queue. Receivers lists receiver
// names; an empty list means no notification. Unknown/disabled names are
// skipped at dispatch time.
type NotifyConfig struct {
	Receivers []string `json:"receivers,omitempty"`
}

// IPACLConfig restricts which source IPs may reach a repository. When Enabled, a
// request whose client IP is not covered by Allow — a list of IPv4/IPv6
// addresses or CIDR blocks — is refused with 403. Enabled with an empty Allow
// denies every request. The client IP is the first X-Forwarded-For hop set by
// the ingress, falling back to the TCP peer address (see audit.ClientIP).
type IPACLConfig struct {
	Enabled bool     `json:"enabled"`
	Allow   []string `json:"allow,omitempty"`
}

// Allowed reports whether ip (a bare address string) matches any Allow entry.
// A client IP that does not parse is denied; a malformed stored entry is
// skipped. Callers should only invoke this when the ACL is Enabled.
func (a IPACLConfig) Allowed(ip string) bool {
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		return false
	}
	addr = addr.Unmap()
	for _, entry := range a.Allow {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if strings.Contains(entry, "/") {
			if pfx, err := netip.ParsePrefix(entry); err == nil && pfx.Contains(addr) {
				return true
			}
			continue
		}
		if got, err := netip.ParseAddr(entry); err == nil && got.Unmap() == addr {
			return true
		}
	}
	return false
}

// Vulnerability policy actions and severity labels.
const (
	VulnActionBlock = "block"
	VulnActionWarn  = "warn"
	VulnActionAudit = "audit"

	SeverityCritical = "critical"
	SeverityHigh     = "high"
	SeverityMedium   = "medium"
	SeverityLow      = "low"
)

// VulnPolicyConfig gates proxy packages by known-vulnerability scan results.
// When enabled, a requested version whose highest advisory severity meets
// Threshold is blocked, warned, or audited per Action. Ignore lists advisory
// ids (CVE/GHSA/OSV) accepted as false-positive or risk-accepted.
// BlockUnscanned blocks a not-yet-scanned version (enforce posture); otherwise
// the request is served and the coordinate is scanned asynchronously.
type VulnPolicyConfig struct {
	Enabled        bool     `json:"enabled"`
	Threshold      string   `json:"threshold,omitempty"` // critical|high|medium|low (default high)
	Action         string   `json:"action,omitempty"`    // block|warn|audit (default audit)
	Ignore         []string `json:"ignore,omitempty"`
	BlockUnscanned bool     `json:"block_unscanned,omitempty"`
}

// EffectiveAction returns Action with the audit default applied.
func (v VulnPolicyConfig) EffectiveAction() string {
	if v.Action == "" {
		return VulnActionAudit
	}
	return v.Action
}

// EffectiveThreshold returns Threshold with the high default applied.
func (v VulnPolicyConfig) EffectiveThreshold() string {
	if v.Threshold == "" {
		return SeverityHigh
	}
	return v.Threshold
}

// LicensePolicyConfig gates proxy packages by their resolved SPDX license(s).
// When enabled, a requested version is evaluated against Deny and Allow: a
// version carrying any license in Deny is blocked/warned/audited per Action;
// if Allow is non-empty, a version carrying any license outside Allow is also
// gated (allow-list mode). BlockUnresolved gates a not-yet-resolved version
// (enforce posture); otherwise the request is served and the coordinate is
// resolved asynchronously. License identifiers are matched case-insensitively.
type LicensePolicyConfig struct {
	Enabled         bool     `json:"enabled"`
	Action          string   `json:"action,omitempty"` // block|warn|audit (default audit)
	Deny            []string `json:"deny,omitempty"`
	Allow           []string `json:"allow,omitempty"`
	BlockUnresolved bool     `json:"block_unresolved,omitempty"`
}

// ArtifactScanPolicyConfig gates stored artifacts by worker-based byte scan
// verdicts. ScannerProfile selects a server-owned scanner profile; repositories
// do not configure raw scanner names or hashes directly.
type ArtifactScanPolicyConfig struct {
	Enabled        bool   `json:"enabled"`
	ScannerProfile string `json:"scanner_profile,omitempty"`
	Threshold      string `json:"threshold,omitempty"` // critical|high|medium|low (default high)
	Action         string `json:"action,omitempty"`    // block|warn|audit (default audit)
	BlockUnscanned bool   `json:"block_unscanned,omitempty"`
}

func (a ArtifactScanPolicyConfig) EffectiveScannerProfile() string {
	if a.ScannerProfile == "" {
		return "grype-default"
	}
	return a.ScannerProfile
}

func (a ArtifactScanPolicyConfig) EffectiveAction() string {
	if a.Action == "" {
		return VulnActionAudit
	}
	return a.Action
}

func (a ArtifactScanPolicyConfig) EffectiveThreshold() string {
	if a.Threshold == "" {
		return SeverityHigh
	}
	return a.Threshold
}

// EffectiveAction returns Action with the audit default applied.
func (l LicensePolicyConfig) EffectiveAction() string {
	if l.Action == "" {
		return VulnActionAudit
	}
	return l.Action
}

// RetentionConfig auto-deletes artifacts that have been idle (not served) for
// IdleTTL, keyed on last_accessed_at (the last-served time). Zero disables it.
// Applies to proxy (cached) and hosted (uploaded) repositories alike; group
// repositories hold no artifacts of their own.
type RetentionConfig struct {
	IdleTTL Duration `json:"idle_ttl,omitempty"`
}

// GroupConfig lists the member repositories of a group repository, in lookup
// order (first hit wins). Only meaningful when the repository type is "group";
// membership invariants that need store access (members exist, same format,
// not themselves groups) are enforced by the API layer.
type GroupConfig struct {
	Members []string `json:"members,omitempty"`
}

// CacheConfig controls proxy caching for a repository.
type CacheConfig struct {
	// Enabled turns caching on. When false a proxy repo passes through to upstream
	// on every request without persisting blobs.
	Enabled bool `json:"enabled"`
	// ArtifactTTL is how long cached immutable artifacts stay fresh before
	// revalidation. Zero means never revalidate (immutable).
	ArtifactTTL Duration `json:"artifact_ttl"`
	// MetadataTTL is how long mutable index documents (maven-metadata.xml, npm
	// packuments, cargo index entries, go @latest) stay fresh.
	MetadataTTL Duration `json:"metadata_ttl"`
	// NegativeTTL is how long 404 results are cached.
	NegativeTTL Duration `json:"negative_ttl"`
	// MaxSizeBytes caps the repository cache size; 0 means unbounded.
	MaxSizeBytes int64 `json:"max_size_bytes"`
	// Eviction is the policy applied when MaxSizeBytes is exceeded. Only "lru".
	Eviction string `json:"eviction"`
}

// AgePolicyConfig gates artifact versions by their upstream release age. The
// primary use is a cooldown window: block versions newer than MinAge to mitigate
// freshly published malicious packages (supply-chain protection).
type AgePolicyConfig struct {
	Enabled bool `json:"enabled"`
	// MinAge requires an upstream release to be at least this old to be served.
	MinAge Duration `json:"min_age"`
	// MaxAge optionally blocks releases older than this. Zero disables it.
	MaxAge Duration `json:"max_age"`
	// Action is "block" (deny) or "warn" (allow but log/metric).
	Action string `json:"action"`
}

// ApprovalConfig gates proxy packages behind an explicit approval decision
// (quarantine). Unapproved packages are blocked with 403 and queued as pending
// approval requests. The decision unit is the whole package; version freshness
// is the age policy's job. Only meaningful for proxy repositories.
type ApprovalConfig struct {
	Enabled bool `json:"enabled"`
	// Mode is "enforce" (block unapproved, default) or "audit" (serve but
	// log/count what enforce would have blocked).
	Mode string `json:"mode,omitempty"`
	// AutoApprove lists path.Match glob patterns of package names that bypass
	// approval, e.g. "@company/*" for an npm scope. Matching is per path
	// segment: "@company/*" matches "@company/lib" but not "@company/a/b".
	AutoApprove []string `json:"auto_approve,omitempty"`
}

// Approval mode constants.
const (
	ModeEnforce = "enforce"
	ModeAudit   = "audit"
)

// EffectiveMode returns the mode with the empty-string default applied.
func (a ApprovalConfig) EffectiveMode() string {
	if a.Mode == "" {
		return ModeEnforce
	}
	return a.Mode
}

// Action constants.
const (
	ActionBlock = "block"
	ActionWarn  = "warn"

	EvictionLRU = "lru"
)

// Default returns a sensible default config for a new repository.
func Default() Config {
	return Config{
		Cache: CacheConfig{
			Enabled:     true,
			MetadataTTL: Duration(15 * time.Minute),
			NegativeTTL: Duration(5 * time.Minute),
			Eviction:    EvictionLRU,
		},
		AgePolicy: AgePolicyConfig{
			Enabled: false,
			Action:  ActionBlock,
		},
	}
}

// Parse decodes and validates a config JSON document, applying defaults for
// omitted sections.
func Parse(raw string) (Config, error) {
	c := Default()
	if strings.TrimSpace(raw) != "" && raw != "{}" {
		if err := json.Unmarshal([]byte(raw), &c); err != nil {
			return Config{}, fmt.Errorf("invalid config json: %w", err)
		}
	}
	return c, c.Validate()
}

// Validate checks invariants.
func (c Config) Validate() error {
	if c.Cache.Eviction != "" && c.Cache.Eviction != EvictionLRU {
		return fmt.Errorf("unsupported eviction %q", c.Cache.Eviction)
	}
	if c.Cache.MaxSizeBytes < 0 {
		return fmt.Errorf("max_size_bytes must be >= 0")
	}
	if c.Retention.IdleTTL < 0 {
		return fmt.Errorf("retention idle_ttl must be >= 0")
	}
	switch c.AgePolicy.Action {
	case "", ActionBlock, ActionWarn:
	default:
		return fmt.Errorf("unsupported age policy action %q", c.AgePolicy.Action)
	}
	switch c.Approval.Mode {
	case "", ModeEnforce, ModeAudit:
	default:
		return fmt.Errorf("unsupported approval mode %q", c.Approval.Mode)
	}
	for _, pat := range c.Approval.AutoApprove {
		if _, err := path.Match(pat, "probe"); err != nil {
			return fmt.Errorf("invalid auto_approve pattern %q: %w", pat, err)
		}
	}
	switch c.Vuln.Action {
	case "", VulnActionBlock, VulnActionWarn, VulnActionAudit:
	default:
		return fmt.Errorf("unsupported vuln action %q", c.Vuln.Action)
	}
	switch c.Vuln.Threshold {
	case "", SeverityCritical, SeverityHigh, SeverityMedium, SeverityLow:
	default:
		return fmt.Errorf("unsupported vuln threshold %q", c.Vuln.Threshold)
	}
	switch c.License.Action {
	case "", VulnActionBlock, VulnActionWarn, VulnActionAudit:
	default:
		return fmt.Errorf("unsupported license action %q", c.License.Action)
	}
	switch c.ArtifactScan.Action {
	case "", VulnActionBlock, VulnActionWarn, VulnActionAudit:
	default:
		return fmt.Errorf("unsupported artifact scan action %q", c.ArtifactScan.Action)
	}
	switch c.ArtifactScan.Threshold {
	case "", SeverityCritical, SeverityHigh, SeverityMedium, SeverityLow:
	default:
		return fmt.Errorf("unsupported artifact scan threshold %q", c.ArtifactScan.Threshold)
	}
	for _, entry := range c.IPACL.Allow {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if strings.Contains(entry, "/") {
			if _, err := netip.ParsePrefix(entry); err != nil {
				return fmt.Errorf("invalid ip_acl allow entry %q: %w", entry, err)
			}
			continue
		}
		if _, err := netip.ParseAddr(entry); err != nil {
			return fmt.Errorf("invalid ip_acl allow entry %q: %w", entry, err)
		}
	}
	return nil
}

// JSON serialises the config.
func (c Config) JSON() (string, error) {
	b, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// Duration is a time.Duration that (un)marshals from human strings, additionally
// supporting day ("d") and week ("w") suffixes (e.g. "3d", "2w", "72h", "30m").
type Duration time.Duration

// D returns the value as a time.Duration.
func (d Duration) D() time.Duration { return time.Duration(d) }

// MarshalJSON renders the duration as a string.
func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Duration(d).String())
}

// UnmarshalJSON accepts a string ("3d") or a number (nanoseconds).
func (d *Duration) UnmarshalJSON(b []byte) error {
	if len(b) > 0 && b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		v, err := ParseDuration(s)
		if err != nil {
			return err
		}
		*d = Duration(v)
		return nil
	}
	var n int64
	if err := json.Unmarshal(b, &n); err != nil {
		return err
	}
	*d = Duration(n)
	return nil
}

// ParseDuration parses Go durations plus "d" (day) and "w" (week) suffixes.
func ParseDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" || s == "0" {
		return 0, nil
	}
	if n := len(s); n >= 2 {
		switch s[n-1] {
		case 'd':
			v, err := strconv.ParseFloat(s[:n-1], 64)
			if err != nil {
				return 0, fmt.Errorf("invalid duration %q: %w", s, err)
			}
			return time.Duration(v * float64(24*time.Hour)), nil
		case 'w':
			v, err := strconv.ParseFloat(s[:n-1], 64)
			if err != nil {
				return 0, fmt.Errorf("invalid duration %q: %w", s, err)
			}
			return time.Duration(v * float64(7*24*time.Hour)), nil
		}
	}
	return time.ParseDuration(s)
}
