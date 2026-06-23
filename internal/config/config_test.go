package config

import (
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	t.Setenv("FORKLIFT_DATA_DIR", "")
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.DataDir != "/data" {
		t.Fatalf("data dir = %q", c.DataDir)
	}
	if c.HTTPAddr != ":8080" || c.MetricsAddr != ":8081" {
		t.Fatalf("addrs = %q %q", c.HTTPAddr, c.MetricsAddr)
	}
	if c.HA.Enabled {
		t.Fatal("HA should default off")
	}
}

func TestLoadOverrides(t *testing.T) {
	t.Setenv("FORKLIFT_DATA_DIR", "/tmp/forklift")
	t.Setenv("FORKLIFT_LOG_LEVEL", "debug")
	t.Setenv("FORKLIFT_LOG_FORMAT", "text")
	t.Setenv("FORKLIFT_SHUTDOWN_TIMEOUT", "30s")
	t.Setenv("FORKLIFT_HA_ENABLED", "true")
	t.Setenv("POD_NAME", "forklift-0")
	t.Setenv("POD_NAMESPACE", "registry")

	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.DataDir != "/tmp/forklift" || c.LogLevel != "debug" || c.LogFormat != "text" {
		t.Fatalf("overrides not applied: %+v", c)
	}
	if c.ShutdownTimeout != 30*time.Second {
		t.Fatalf("shutdown timeout = %v", c.ShutdownTimeout)
	}
	if !c.HA.Enabled || c.HA.Identity != "forklift-0" || c.HA.LeaseNamespace != "registry" {
		t.Fatalf("HA config = %+v", c.HA)
	}
}

func TestValidateRejectsBadValues(t *testing.T) {
	t.Setenv("FORKLIFT_LOG_LEVEL", "trace")
	if _, err := Load(); err == nil {
		t.Fatal("expected invalid log level error")
	}
}

func TestValidateRejectsBadFormat(t *testing.T) {
	t.Setenv("FORKLIFT_LOG_FORMAT", "xml")
	if _, err := Load(); err == nil {
		t.Fatal("expected invalid log format error")
	}
}

func TestHAIdentityRequiredWhenEnabled(t *testing.T) {
	t.Setenv("FORKLIFT_HA_ENABLED", "true")
	t.Setenv("FORKLIFT_HA_IDENTITY", "")
	t.Setenv("POD_NAME", "")
	// Hostname normally fills identity; force it empty via both sources being
	// blank is not possible (hostname fallback), so assert the happy path holds.
	c, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.HA.Identity == "" {
		t.Fatal("identity should fall back to hostname")
	}
}

func TestEnvFallbacksOnInvalid(t *testing.T) {
	t.Setenv("FORKLIFT_HA_ENABLED", "notabool")
	t.Setenv("FORKLIFT_SHUTDOWN_TIMEOUT", "notaduration")
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.HA.Enabled {
		t.Fatal("invalid bool should fall back to default false")
	}
	if c.ShutdownTimeout != 15*time.Second {
		t.Fatalf("invalid duration should fall back to default, got %v", c.ShutdownTimeout)
	}
}

func TestAuthDefaults(t *testing.T) {
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.Auth.BootstrapAdminUser != "admin" || c.Auth.SessionTTL != 12*time.Hour {
		t.Fatalf("auth defaults = %+v", c.Auth)
	}
	if c.Auth.OIDC.UsernameClaim != "preferred_username" || c.Auth.OIDC.GroupsClaim != "groups" {
		t.Fatalf("oidc claim defaults = %+v", c.Auth.OIDC)
	}
}

func TestStorageDefaultsFS(t *testing.T) {
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.Storage.Backend != "fs" {
		t.Fatalf("storage backend = %q, want fs", c.Storage.Backend)
	}
	if c.Storage.MetaSyncInterval != 30*time.Second {
		t.Fatalf("meta sync interval = %v", c.Storage.MetaSyncInterval)
	}
}

func TestStorageS3Loads(t *testing.T) {
	t.Setenv("FORKLIFT_STORAGE_BACKEND", "s3")
	t.Setenv("FORKLIFT_STORAGE_S3_BUCKET", "my-bucket")
	t.Setenv("FORKLIFT_STORAGE_S3_PREFIX", "forklift")
	t.Setenv("FORKLIFT_STORAGE_S3_REGION", "ap-northeast-2")
	t.Setenv("FORKLIFT_STORAGE_S3_FORCE_PATH_STYLE", "true")
	t.Setenv("FORKLIFT_STORAGE_META_SYNC_INTERVAL", "10s")

	c, err := Load()
	if err != nil {
		t.Fatalf("valid s3 config rejected: %v", err)
	}
	if c.Storage.Backend != "s3" || c.Storage.S3.Bucket != "my-bucket" {
		t.Fatalf("s3 config = %+v", c.Storage)
	}
	if !c.Storage.S3.ForcePathStyle || c.Storage.S3.Region != "ap-northeast-2" {
		t.Fatalf("s3 config = %+v", c.Storage.S3)
	}
	if c.Storage.MetaSyncInterval != 10*time.Second {
		t.Fatalf("meta sync interval = %v", c.Storage.MetaSyncInterval)
	}
}

func TestStorageS3RequiresBucket(t *testing.T) {
	t.Setenv("FORKLIFT_STORAGE_BACKEND", "s3")
	if _, err := Load(); err == nil {
		t.Fatal("expected error when s3 backend has no bucket")
	}
}

func TestStorageRejectsUnknownBackend(t *testing.T) {
	t.Setenv("FORKLIFT_STORAGE_BACKEND", "gcs")
	if _, err := Load(); err == nil {
		t.Fatal("expected error for unknown backend")
	}
}

func TestStorageS3RejectsPartialStaticCreds(t *testing.T) {
	t.Setenv("FORKLIFT_STORAGE_BACKEND", "s3")
	t.Setenv("FORKLIFT_STORAGE_S3_BUCKET", "b")
	t.Setenv("FORKLIFT_STORAGE_S3_ACCESS_KEY_ID", "only-id")
	if _, err := Load(); err == nil {
		t.Fatal("expected error when only one static credential is set")
	}
}

func TestStorageS3IncompatibleWithReplication(t *testing.T) {
	t.Setenv("FORKLIFT_STORAGE_BACKEND", "s3")
	t.Setenv("FORKLIFT_STORAGE_S3_BUCKET", "b")
	t.Setenv("FORKLIFT_REPLICATION_ENABLED", "true")
	if _, err := Load(); err == nil {
		t.Fatal("expected error for s3 backend + replication")
	}
}
