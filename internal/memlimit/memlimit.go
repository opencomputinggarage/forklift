// Package memlimit sets the Go runtime's soft memory limit from the
// container's cgroup memory limit. Without it the garbage collector (GOGC=100)
// happily lets the heap grow to twice the live set, which under a request
// burst pushes RSS past the Kubernetes limit and gets the pod OOMKilled; with
// a soft limit the collector runs harder as usage approaches the cap instead.
package memlimit

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"runtime/debug"
	"strconv"
)

// ratio leaves headroom below the hard cgroup limit for non-heap memory the
// runtime does not track (goroutine stacks, mmapped files, cgo).
const ratio = 0.9

// minLimit guards against absurdly small (or misread) limits that would make
// the collector spin permanently.
const minLimit = 64 << 20

// Apply sets the runtime soft memory limit to ratio × the cgroup memory limit.
// It is a no-op when GOMEMLIMIT is set explicitly (the runtime already honors
// it), when no cgroup limit is configured, or outside a container.
func Apply(log *slog.Logger) {
	if os.Getenv("GOMEMLIMIT") != "" {
		return
	}
	limit, ok := cgroupLimit("/sys/fs/cgroup")
	if !ok {
		return
	}
	soft := int64(float64(limit) * ratio)
	if soft < minLimit {
		soft = minLimit
	}
	debug.SetMemoryLimit(soft)
	log.Info("memory limit applied from cgroup", "cgroup_limit_bytes", limit, "gomemlimit_bytes", soft)
}

// cgroupLimit reads the container memory limit from the cgroup filesystem
// rooted at root, supporting both v2 (memory.max) and v1
// (memory/memory.limit_in_bytes). Returns false when no finite limit is set.
func cgroupLimit(root string) (int64, bool) {
	for _, p := range []string{
		filepath.Join(root, "memory.max"),
		filepath.Join(root, "memory", "memory.limit_in_bytes"),
	} {
		raw, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		s := string(bytes.TrimSpace(raw))
		if s == "max" { // cgroup v2: no limit configured
			return 0, false
		}
		n, err := strconv.ParseInt(s, 10, 64)
		if err != nil || n <= 0 {
			return 0, false
		}
		// cgroup v1 reports "unlimited" as a huge page-rounded value.
		if n >= int64(1)<<62 {
			return 0, false
		}
		return n, true
	}
	return 0, false
}
