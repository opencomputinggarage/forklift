package packagecoord

import "testing"

func TestFromArtifact(t *testing.T) {
	tests := []struct {
		name      string
		format    string
		path      string
		version   string
		pkg       string
		ecosystem string
		system    string
		purl      string
	}{
		{
			name:      "npm tarball",
			format:    "npm",
			path:      "axios/-/axios-0.21.1.tgz",
			pkg:       "axios",
			ecosystem: "npm",
			system:    "npm",
			purl:      "pkg:npm/axios@0.21.1",
		},
		{
			name:      "scoped npm tarball",
			format:    "npm",
			path:      "@scope/name/-/name-1.2.3.tgz",
			pkg:       "@scope/name",
			ecosystem: "npm",
			system:    "npm",
			purl:      "pkg:npm/%40scope/name@1.2.3",
		},
		{
			name:      "maven jar",
			format:    "maven",
			path:      "com/google/guava/guava/31.0/guava-31.0.jar",
			pkg:       "com.google.guava:guava",
			ecosystem: "Maven",
			system:    "maven",
			purl:      "pkg:maven/com.google.guava/guava@31.0",
		},
		{
			name:      "maven metadata has no version purl",
			format:    "maven",
			path:      "com/google/guava/guava/maven-metadata.xml",
			pkg:       "com.google.guava:guava",
			ecosystem: "Maven",
			system:    "maven",
		},
		{
			name:      "cargo crate",
			format:    "cargo",
			path:      "api/v1/crates/serde/1.0.197/download",
			pkg:       "serde",
			ecosystem: "crates.io",
			system:    "cargo",
			purl:      "pkg:cargo/serde@1.0.197",
		},
		{
			name:      "cargo index has no version purl",
			format:    "cargo",
			path:      "se/rd/serde",
			pkg:       "serde",
			ecosystem: "crates.io",
			system:    "cargo",
		},
		{
			name:      "cargo non-download api path has no version purl",
			format:    "cargo",
			path:      "api/v1/crates/serde/1.0.197/metadata",
			pkg:       "serde",
			ecosystem: "crates.io",
			system:    "cargo",
		},
		{
			name:      "go module zip",
			format:    "go",
			path:      "github.com/gin-gonic/gin/@v/v1.9.0.zip",
			pkg:       "github.com/gin-gonic/gin",
			ecosystem: "Go",
			system:    "go",
			purl:      "pkg:golang/github.com/gin-gonic/gin@v1.9.0",
		},
		{
			name:      "go version list has no version purl",
			format:    "go",
			path:      "github.com/gin-gonic/gin/@v/list",
			pkg:       "github.com/gin-gonic/gin",
			ecosystem: "Go",
			system:    "go",
		},
		{
			name:      "pypi wheel",
			format:    "pypi",
			path:      "packages/ab/cd/requests-2.31.0-py3-none-any.whl",
			pkg:       "requests",
			ecosystem: "PyPI",
			system:    "pypi",
			purl:      "pkg:pypi/requests@2.31.0",
		},
		{
			name:      "pypi pep 658 metadata has no version purl",
			format:    "pypi",
			path:      "packages/ab/cd/requests-2.31.0-py3-none-any.whl.metadata",
			pkg:       "requests",
			ecosystem: "PyPI",
			system:    "pypi",
		},
		{
			name:      "explicit version wins",
			format:    "npm",
			path:      "axios/-/axios-0.21.1.tgz",
			version:   "0.21.2",
			pkg:       "axios",
			ecosystem: "npm",
			system:    "npm",
			purl:      "pkg:npm/axios@0.21.2",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FromArtifact(tt.format, tt.path, tt.version)
			if got.PackageName != tt.pkg || got.Ecosystem != tt.ecosystem || got.DepsDevSystem != tt.system || got.PURL != tt.purl {
				t.Fatalf("coordinate = %+v", got)
			}
		})
	}
}
