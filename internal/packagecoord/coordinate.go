package packagecoord

import (
	"net/url"
	"path"
	"strings"
)

// Coordinate is the best-effort package identity derived from a repository
// artifact path. Empty PackageName means the path is metadata or unsupported.
type Coordinate struct {
	Ecosystem     string
	DepsDevSystem string
	PackageName   string
	Version       string
	PURL          string
}

// FromArtifact derives a package coordinate from Forklift repository metadata.
// explicitVersion wins over the path-derived version when present.
func FromArtifact(format, artifactPath, explicitVersion string) Coordinate {
	c := Coordinate{
		Ecosystem:     OSVEcosystem(format),
		DepsDevSystem: DepsDevSystem(format),
	}
	if c.Ecosystem == "" && c.DepsDevSystem == "" {
		return c
	}
	switch format {
	case "maven":
		c.PackageName = MavenPackage(artifactPath)
		c.Version = MavenVersion(artifactPath)
	case "npm":
		c.PackageName = NPMPackage(artifactPath)
		c.Version = NPMVersion(artifactPath)
	case "cargo":
		c.PackageName = CargoPackage(artifactPath)
		c.Version = CargoVersion(artifactPath)
	case "go":
		c.PackageName = GoPackage(artifactPath)
		c.Version = GoVersion(artifactPath)
	case "pypi":
		c.PackageName = PyPIPackageFromFilename(path.Base(artifactPath))
		c.Version = PyPIVersion(path.Base(artifactPath))
	}
	if explicitVersion != "" {
		c.Version = explicitVersion
	}
	c.PURL = PURL(format, c.PackageName, c.Version)
	return c
}

// OSVEcosystem maps a Forklift repository format to its OSV ecosystem name.
func OSVEcosystem(format string) string {
	switch format {
	case "maven":
		return "Maven"
	case "npm":
		return "npm"
	case "cargo":
		return "crates.io"
	case "go":
		return "Go"
	case "pypi":
		return "PyPI"
	default:
		return ""
	}
}

// DepsDevSystem maps a Forklift repository format to its deps.dev system name.
func DepsDevSystem(format string) string {
	switch format {
	case "maven":
		return "maven"
	case "npm":
		return "npm"
	case "cargo":
		return "cargo"
	case "go":
		return "go"
	case "pypi":
		return "pypi"
	default:
		return ""
	}
}

// PURL returns a Package URL for supported package coordinates.
func PURL(format, pkg, version string) string {
	if pkg == "" || version == "" {
		return ""
	}
	ver := url.PathEscape(version)
	switch format {
	case "maven":
		group, artifact, ok := strings.Cut(pkg, ":")
		if !ok || group == "" || artifact == "" {
			return ""
		}
		return "pkg:maven/" + url.PathEscape(group) + "/" + url.PathEscape(artifact) + "@" + ver
	case "npm":
		if strings.HasPrefix(pkg, "@") {
			scope, name, ok := strings.Cut(pkg, "/")
			if !ok || scope == "" || name == "" {
				return ""
			}
			scope = strings.ReplaceAll(url.PathEscape(scope), "@", "%40")
			return "pkg:npm/" + scope + "/" + url.PathEscape(name) + "@" + ver
		}
		return "pkg:npm/" + url.PathEscape(pkg) + "@" + ver
	case "cargo":
		return "pkg:cargo/" + url.PathEscape(pkg) + "@" + ver
	case "go":
		return "pkg:golang/" + escapePath(pkg) + "@" + ver
	case "pypi":
		return "pkg:pypi/" + url.PathEscape(pkg) + "@" + ver
	default:
		return ""
	}
}

// MavenPackage extracts group:artifact from a Maven repository path.
func MavenPackage(p string) string {
	parts := strings.Split(strings.Trim(p, "/"), "/")
	if strings.HasPrefix(path.Base(p), "maven-metadata.xml") {
		parts = parts[:len(parts)-1]
		if len(parts) > 0 && strings.HasSuffix(parts[len(parts)-1], "-SNAPSHOT") {
			parts = parts[:len(parts)-1]
		}
	} else if len(parts) >= 2 {
		parts = parts[:len(parts)-2]
	} else {
		return ""
	}
	if len(parts) < 2 {
		return ""
	}
	return strings.Join(parts[:len(parts)-1], ".") + ":" + parts[len(parts)-1]
}

// MavenVersion extracts the version segment from a Maven repository path.
func MavenVersion(p string) string {
	if strings.HasPrefix(path.Base(p), "maven-metadata.xml") {
		return ""
	}
	parts := strings.Split(strings.Trim(p, "/"), "/")
	if len(parts) >= 2 {
		return parts[len(parts)-2]
	}
	return ""
}

// NPMPackage extracts the package name from an npm path.
func NPMPackage(p string) string {
	if i := strings.Index(p, "/-/"); i >= 0 {
		p = p[:i]
	}
	p = strings.ReplaceAll(p, "%2f", "/")
	p = strings.ReplaceAll(p, "%2F", "/")
	return strings.ToLower(strings.Trim(p, "/"))
}

// NPMVersion extracts the version from an npm tarball path.
func NPMVersion(p string) string {
	i := strings.Index(p, "/-/")
	if i < 0 {
		return ""
	}
	pkg := NPMPackage(p)
	file := strings.ToLower(path.Base(p))
	base := path.Base(pkg)
	v, ok := strings.CutPrefix(strings.TrimSuffix(file, ".tgz"), base+"-")
	if !ok || v == "" || !strings.HasSuffix(file, ".tgz") {
		return ""
	}
	return v
}

// CargoPackage extracts a crate name from Cargo sparse-registry paths.
func CargoPackage(p string) string {
	if p == "config.json" {
		return ""
	}
	if i := strings.Index(p, "api/v1/crates/"); i >= 0 {
		crate, _, _ := strings.Cut(p[i+len("api/v1/crates/"):], "/")
		return strings.ToLower(crate)
	}
	return strings.ToLower(path.Base(p))
}

// CargoVersion extracts a crate version from Cargo download paths.
func CargoVersion(p string) string {
	if i := strings.Index(p, "api/v1/crates/"); i >= 0 {
		rest := strings.TrimSuffix(p[i+len("api/v1/crates/"):], "/download")
		parts := strings.Split(rest, "/")
		if len(parts) == 2 {
			return parts[1]
		}
	}
	return ""
}

// GoPackage extracts the module path from a GOPROXY protocol path.
func GoPackage(p string) string {
	if i := strings.Index(p, "/@v/"); i >= 0 {
		return p[:i]
	}
	if mod, ok := strings.CutSuffix(p, "/@latest"); ok {
		return mod
	}
	return ""
}

// GoVersion extracts the version from a GOPROXY protocol artifact path.
func GoVersion(p string) string {
	idx := strings.Index(p, "/@v/")
	if idx < 0 {
		return ""
	}
	rest := p[idx+len("/@v/"):]
	for _, ext := range []string{".info", ".mod", ".zip"} {
		if strings.HasSuffix(rest, ext) {
			return strings.TrimSuffix(rest, ext)
		}
	}
	return ""
}

// PyPIPackageFromFilename extracts a normalized project name from a distribution filename.
func PyPIPackageFromFilename(f string) string {
	if strings.HasSuffix(f, ".metadata") {
		return PyPIPackageFromFilename(strings.TrimSuffix(f, ".metadata"))
	}
	switch {
	case strings.HasSuffix(f, ".whl"):
		parts := strings.SplitN(strings.TrimSuffix(f, ".whl"), "-", 3)
		if len(parts) >= 2 {
			return normalizePyPI(parts[0])
		}
	case strings.HasSuffix(f, ".tar.gz"):
		if stem := strings.TrimSuffix(f, ".tar.gz"); strings.LastIndex(stem, "-") > 0 {
			return normalizePyPI(stem[:strings.LastIndex(stem, "-")])
		}
	case strings.HasSuffix(f, ".zip"):
		if stem := strings.TrimSuffix(f, ".zip"); strings.LastIndex(stem, "-") > 0 {
			return normalizePyPI(stem[:strings.LastIndex(stem, "-")])
		}
	}
	return ""
}

// PyPIVersion extracts the version from a distribution filename.
func PyPIVersion(f string) string {
	switch {
	case strings.HasSuffix(f, ".whl"):
		parts := strings.Split(strings.TrimSuffix(f, ".whl"), "-")
		if len(parts) >= 2 {
			return parts[1]
		}
	case strings.HasSuffix(f, ".tar.gz"):
		stem := strings.TrimSuffix(f, ".tar.gz")
		if i := strings.LastIndex(stem, "-"); i >= 0 {
			return stem[i+1:]
		}
	case strings.HasSuffix(f, ".zip"):
		stem := strings.TrimSuffix(f, ".zip")
		if i := strings.LastIndex(stem, "-"); i >= 0 {
			return stem[i+1:]
		}
	}
	return ""
}

func normalizePyPI(s string) string {
	return strings.ToLower(strings.NewReplacer("_", "-", ".", "-").Replace(s))
}

func escapePath(p string) string {
	parts := strings.Split(p, "/")
	for i := range parts {
		parts[i] = url.PathEscape(parts[i])
	}
	return strings.Join(parts, "/")
}
