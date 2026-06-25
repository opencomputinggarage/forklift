// Package license resolves the declared license(s) of a package coordinate
// (system, name, version) against an external metadata source (deps.dev). It
// performs coordinate matching only: the directly requested version is
// resolved, not its transitive dependencies and not the artifact bytes.
package license

import "context"

// Result is the resolved license information for one coordinate: the SPDX
// license expressions reported by the source. Licenses is empty when the source
// reports no license (unknown / unlicensed).
type Result struct {
	Licenses []string
}

// Resolver looks up the declared licenses for a coordinate. Source names the
// data source (e.g. "deps.dev"), recorded on each resolution so the report
// attributes its data and stays meaningful as more sources are added.
type Resolver interface {
	Resolve(ctx context.Context, system, pkg, version string) (Result, error)
	Source() string
}
