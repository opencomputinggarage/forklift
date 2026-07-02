package memlimit

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCgroupLimitV2(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "memory.max"), "268435456\n")
	n, ok := cgroupLimit(root)
	if !ok || n != 268435456 {
		t.Fatalf("cgroupLimit = %d, %v; want 268435456, true", n, ok)
	}
}

func TestCgroupLimitV2Unlimited(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "memory.max"), "max\n")
	if _, ok := cgroupLimit(root); ok {
		t.Fatal("unlimited v2 cgroup should report no limit")
	}
}

func TestCgroupLimitV1(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "memory", "memory.limit_in_bytes"), "536870912\n")
	n, ok := cgroupLimit(root)
	if !ok || n != 536870912 {
		t.Fatalf("cgroupLimit = %d, %v; want 536870912, true", n, ok)
	}
}

func TestCgroupLimitV1Unlimited(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "memory", "memory.limit_in_bytes"), "9223372036854771712\n")
	if _, ok := cgroupLimit(root); ok {
		t.Fatal("unlimited v1 cgroup should report no limit")
	}
}

func TestCgroupLimitAbsent(t *testing.T) {
	if _, ok := cgroupLimit(t.TempDir()); ok {
		t.Fatal("missing cgroup files should report no limit")
	}
}
