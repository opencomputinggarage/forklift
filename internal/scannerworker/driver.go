// Package scannerworker contains the untrusted artifact execution side of
// artifact scanning. It must not be imported by the forklift server.
package scannerworker

import (
	"context"
	"fmt"
	"sort"

	"github.com/younsl/o/box/kubernetes/forklift/internal/artifactscan"
)

// Driver runs one scanner implementation against a prepared artifact.
type Driver interface {
	Name() string
	Version(ctx context.Context) (string, error)
	Capability(ctx context.Context) (artifactscan.ScannerCapability, error)
	Scan(ctx context.Context, artifact PreparedArtifact) (artifactscan.Result, error)
}

// SBOMGenerator is implemented by drivers that can produce package inventory in
// addition to vulnerability findings.
type SBOMGenerator interface {
	GenerateSBOM(ctx context.Context, artifact PreparedArtifact) (artifactscan.SBOM, error)
}

// Registry maps scanner names to worker-side drivers.
type Registry struct {
	drivers map[string]Driver
}

// NewRegistry builds a registry from the provided drivers.
func NewRegistry(drivers ...Driver) (*Registry, error) {
	r := &Registry{drivers: map[string]Driver{}}
	for _, d := range drivers {
		if d == nil {
			continue
		}
		name := d.Name()
		if name == "" {
			return nil, fmt.Errorf("scanner driver with empty name")
		}
		if _, exists := r.drivers[name]; exists {
			return nil, fmt.Errorf("duplicate scanner driver %q", name)
		}
		r.drivers[name] = d
	}
	return r, nil
}

// Get returns a driver by scanner name.
func (r *Registry) Get(name string) (Driver, bool) {
	if r == nil {
		return nil, false
	}
	d, ok := r.drivers[name]
	return d, ok
}

// Names returns registered scanner names in stable order.
func (r *Registry) Names() []string {
	if r == nil {
		return nil
	}
	out := make([]string, 0, len(r.drivers))
	for name := range r.drivers {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// Capabilities returns metadata for every registered driver in stable order.
func (r *Registry) Capabilities(ctx context.Context) ([]artifactscan.ScannerCapability, error) {
	if r == nil {
		return nil, nil
	}
	names := r.Names()
	out := make([]artifactscan.ScannerCapability, 0, len(names))
	for _, name := range names {
		capability, err := r.drivers[name].Capability(ctx)
		if err != nil {
			return nil, fmt.Errorf("scanner %q capability: %w", name, err)
		}
		if capability.Name == "" {
			capability.Name = name
		}
		out = append(out, capability)
	}
	return out, nil
}
