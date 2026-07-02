package scannerworker

import (
	"context"
	"reflect"
	"testing"

	"github.com/younsl/o/box/kubernetes/forklift/internal/artifactscan"
)

type fakeDriver struct {
	name string
}

func (f fakeDriver) Name() string { return f.name }

func (f fakeDriver) Version(context.Context) (string, error) { return "test", nil }

func (f fakeDriver) Capability(context.Context) (artifactscan.ScannerCapability, error) {
	return artifactscan.ScannerCapability{Name: f.name, Version: "test"}, nil
}

func (f fakeDriver) Scan(context.Context, PreparedArtifact) (artifactscan.Result, error) {
	return artifactscan.Result{Scanner: f.name}, nil
}

func TestRegistry(t *testing.T) {
	reg, err := NewRegistry(fakeDriver{name: "z"}, fakeDriver{name: "a"})
	if err != nil {
		t.Fatalf("new registry: %v", err)
	}
	if got := reg.Names(); !reflect.DeepEqual(got, []string{"a", "z"}) {
		t.Fatalf("names = %v", got)
	}
	if _, ok := reg.Get("a"); !ok {
		t.Fatal("registered driver not found")
	}
	if _, ok := reg.Get("missing"); ok {
		t.Fatal("missing driver found")
	}
	caps, err := reg.Capabilities(context.Background())
	if err != nil {
		t.Fatalf("capabilities: %v", err)
	}
	if len(caps) != 2 || caps[0].Name != "a" || caps[1].Name != "z" {
		t.Fatalf("capabilities = %+v", caps)
	}
}

func TestRegistryRejectsInvalidDrivers(t *testing.T) {
	if _, err := NewRegistry(fakeDriver{}); err == nil {
		t.Fatal("empty driver name accepted")
	}
	if _, err := NewRegistry(fakeDriver{name: "grype"}, fakeDriver{name: "grype"}); err == nil {
		t.Fatal("duplicate driver accepted")
	}
}
