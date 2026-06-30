package repoconfig

import (
	"testing"
	"time"
)

func TestParseDuration(t *testing.T) {
	cases := map[string]time.Duration{
		"":     0,
		"0":    0,
		"30m":  30 * time.Minute,
		"72h":  72 * time.Hour,
		"3d":   3 * 24 * time.Hour,
		"2w":   2 * 7 * 24 * time.Hour,
		"1.5d": 36 * time.Hour,
	}
	for in, want := range cases {
		got, err := ParseDuration(in)
		if err != nil {
			t.Fatalf("ParseDuration(%q): %v", in, err)
		}
		if got != want {
			t.Fatalf("ParseDuration(%q) = %v, want %v", in, got, want)
		}
	}
	if _, err := ParseDuration("nonsense"); err == nil {
		t.Fatal("expected error for invalid duration")
	}
}

func TestParseAppliesDefaults(t *testing.T) {
	c, err := Parse("")
	if err != nil {
		t.Fatal(err)
	}
	if !c.Cache.Enabled {
		t.Fatal("cache should default enabled")
	}
	if c.Cache.MetadataTTL.D() != 15*time.Minute {
		t.Fatalf("metadata ttl default = %v", c.Cache.MetadataTTL.D())
	}
	if c.AgePolicy.Action != ActionBlock {
		t.Fatalf("age policy action default = %q", c.AgePolicy.Action)
	}
}

func TestParseRoundTrip(t *testing.T) {
	in := `{"cache":{"enabled":true,"artifact_ttl":"1h","max_size_bytes":1048576,"eviction":"lru"},"age_policy":{"enabled":true,"min_age":"3d","action":"block"}}`
	c, err := Parse(in)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if c.Cache.MaxSizeBytes != 1048576 {
		t.Fatalf("max size = %d", c.Cache.MaxSizeBytes)
	}
	if c.AgePolicy.MinAge.D() != 3*24*time.Hour {
		t.Fatalf("min age = %v", c.AgePolicy.MinAge.D())
	}
	out, err := c.JSON()
	if err != nil {
		t.Fatal(err)
	}
	c2, err := Parse(out)
	if err != nil {
		t.Fatalf("reparse: %v", err)
	}
	if c2.AgePolicy.MinAge.D() != c.AgePolicy.MinAge.D() {
		t.Fatal("round trip lost min_age")
	}
}

func TestValidate(t *testing.T) {
	bad := Config{Cache: CacheConfig{Eviction: "fifo"}}
	if err := bad.Validate(); err == nil {
		t.Fatal("expected eviction validation error")
	}
	bad = Config{Cache: CacheConfig{MaxSizeBytes: -1}}
	if err := bad.Validate(); err == nil {
		t.Fatal("expected negative size error")
	}
	bad = Config{AgePolicy: AgePolicyConfig{Action: "drop"}}
	if err := bad.Validate(); err == nil {
		t.Fatal("expected action validation error")
	}
	bad = Config{Approval: ApprovalConfig{Mode: "quarantine"}}
	if err := bad.Validate(); err == nil {
		t.Fatal("expected approval mode validation error")
	}
	bad = Config{Approval: ApprovalConfig{AutoApprove: []string{"[invalid"}}}
	if err := bad.Validate(); err == nil {
		t.Fatal("expected auto_approve pattern validation error")
	}
	bad = Config{Retention: RetentionConfig{IdleTTL: -1}}
	if err := bad.Validate(); err == nil {
		t.Fatal("expected negative idle_ttl error")
	}
	bad = Config{ArtifactScan: ArtifactScanPolicyConfig{Action: "drop"}}
	if err := bad.Validate(); err == nil {
		t.Fatal("expected artifact scan action validation error")
	}
	bad = Config{ArtifactScan: ArtifactScanPolicyConfig{Threshold: "none"}}
	if err := bad.Validate(); err == nil {
		t.Fatal("expected artifact scan threshold validation error")
	}
}

func TestArtifactScanDefaults(t *testing.T) {
	cfg := Default().ArtifactScan
	if got := cfg.EffectiveScanner(); got != "grype" {
		t.Fatalf("scanner=%q, want grype", got)
	}
	if got := cfg.EffectiveAction(); got != VulnActionAudit {
		t.Fatalf("action=%q, want audit", got)
	}
	if got := cfg.EffectiveThreshold(); got != SeverityHigh {
		t.Fatalf("threshold=%q, want high", got)
	}
}

func TestRetentionConfigRoundTrip(t *testing.T) {
	c, err := Parse(`{"retention":{"idle_ttl":"7d"}}`)
	if err != nil {
		t.Fatal(err)
	}
	if c.Retention.IdleTTL.D() != 7*24*time.Hour {
		t.Fatalf("idle_ttl = %v, want 168h", c.Retention.IdleTTL.D())
	}
	raw, err := c.JSON()
	if err != nil {
		t.Fatal(err)
	}
	again, err := Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if again.Retention.IdleTTL != c.Retention.IdleTTL {
		t.Fatalf("round-trip idle_ttl = %v, want %v", again.Retention.IdleTTL, c.Retention.IdleTTL)
	}
}

func TestApprovalConfig(t *testing.T) {
	in := `{"approval":{"enabled":true,"mode":"audit","auto_approve":["@company/*","left-*"]}}`
	c, err := Parse(in)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !c.Approval.Enabled || c.Approval.EffectiveMode() != ModeAudit {
		t.Fatalf("approval = %+v", c.Approval)
	}
	if len(c.Approval.AutoApprove) != 2 {
		t.Fatalf("auto_approve = %v", c.Approval.AutoApprove)
	}
	out, err := c.JSON()
	if err != nil {
		t.Fatal(err)
	}
	c2, err := Parse(out)
	if err != nil {
		t.Fatalf("reparse: %v", err)
	}
	if c2.Approval.Mode != ModeAudit || len(c2.Approval.AutoApprove) != 2 {
		t.Fatal("round trip lost approval config")
	}
	// Defaults: disabled, effective mode enforce.
	d, err := Parse("")
	if err != nil {
		t.Fatal(err)
	}
	if d.Approval.Enabled || d.Approval.EffectiveMode() != ModeEnforce {
		t.Fatalf("approval default = %+v", d.Approval)
	}
}

func TestIPACLAllowed(t *testing.T) {
	acl := IPACLConfig{Enabled: true, Allow: []string{"203.0.113.5", "10.0.0.0/16", "2001:db8::/32", "  ", "bogus"}}
	cases := map[string]bool{
		"203.0.113.5":        true,  // exact IPv4
		"203.0.113.6":        false, // outside
		"10.0.5.7":           true,  // inside CIDR
		"10.1.0.1":           false, // outside CIDR
		"2001:db8::1":        true,  // inside IPv6 CIDR
		"2001:dead::1":       false, // outside IPv6 CIDR
		"::ffff:203.0.113.5": true,  // IPv4-mapped form of an allowed v4
		"not-an-ip":          false, // unparseable client ip denied
	}
	for ip, want := range cases {
		if got := acl.Allowed(ip); got != want {
			t.Errorf("Allowed(%q) = %v, want %v", ip, got, want)
		}
	}
}

func TestIPACLValidate(t *testing.T) {
	good := Default()
	good.IPACL = IPACLConfig{Enabled: true, Allow: []string{"10.0.0.0/8", "203.0.113.1", " "}}
	if err := good.Validate(); err != nil {
		t.Fatalf("valid ip_acl rejected: %v", err)
	}
	bad := Default()
	bad.IPACL = IPACLConfig{Enabled: true, Allow: []string{"10.0.0.0/99"}}
	if err := bad.Validate(); err == nil {
		t.Fatal("invalid CIDR accepted")
	}
	bad2 := Default()
	bad2.IPACL = IPACLConfig{Allow: []string{"not-an-ip"}}
	if err := bad2.Validate(); err == nil {
		t.Fatal("invalid address accepted")
	}
}
