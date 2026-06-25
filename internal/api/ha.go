package api

import (
	"context"
	"net/http"
)

// HAStatus is the high-availability snapshot shown in the admin management
// console: the active storage/HA topology, this pod's identity and role, the
// current Lease holder, and (in s3 mode) the fencing token.
type HAStatus struct {
	// Enabled reports whether leader election is active (HA mode).
	Enabled bool `json:"enabled"`
	// Mode is the topology: single, shared-volume, replication, or object-storage.
	Mode string `json:"mode"`
	// Backend is the storage backend: fs or s3.
	Backend string `json:"backend"`
	// StorageEndpoint is the address artifacts live at: the object-storage
	// bucket/endpoint (s3) or the block-storage data directory (fs).
	StorageEndpoint string `json:"storage_endpoint,omitempty"`
	// Identity is this pod's leader-election identity (its pod name).
	Identity string `json:"identity"`
	// Leader is the current Lease holder's identity ("" if none/unknown).
	Leader string `json:"leader"`
	// IsLeader reports whether this pod currently holds leadership.
	IsLeader bool `json:"is_leader"`
	// Role is "leader" or "standby".
	Role string `json:"role"`
	// LeaseName is the leader-election Lease object name (HA mode).
	LeaseName string `json:"lease_name,omitempty"`
	// FencingToken is the Lease transition count guarding s3 metadata writes.
	FencingToken int64 `json:"fencing_token,omitempty"`
	// StartedAt is this process's start time (RFC3339), for uptime display.
	StartedAt string `json:"started_at,omitempty"`
	// Version is this pod's forklift version, shown on the architecture diagram.
	Version string `json:"version,omitempty"`
}

// SetHAStatus injects the provider that assembles live HA status. Wired from
// main once leader election and the storage backend are known.
func (h *Handler) SetHAStatus(fn func(context.Context) HAStatus) { h.haStatus = fn }

// SetHAStepDown injects the manual-failover trigger: it asks this instance to
// release leadership, reporting whether it was the leader. Wired from main only
// in HA mode; left nil for single-instance, where there is nothing to fail over.
func (h *Handler) SetHAStepDown(fn func() bool) { h.haStepDown = fn }

// getHAStatus reports HA/leadership status. Registered under the admin-only
// route group. When no provider is wired (e.g. tests) it reports a
// single-instance leader.
func (h *Handler) getHAStatus(w http.ResponseWriter, r *http.Request) {
	if h.haStatus == nil {
		writeJSON(w, http.StatusOK, HAStatus{Mode: "single", Backend: "fs", IsLeader: true, Role: "leader"})
		return
	}
	writeJSON(w, http.StatusOK, h.haStatus(r.Context()))
}

// stepDownHA triggers a manual failover: the current leader releases its Lease
// so a standby takes over. Admin-only. Admin traffic is routed to the leader,
// so the request normally lands there; if it lands on a standby (or HA is off)
// there is no leadership to release and it reports 409.
func (h *Handler) stepDownHA(w http.ResponseWriter, _ *http.Request) {
	if h.haStepDown == nil {
		writeError(w, http.StatusConflict, "manual failover is unavailable in single-instance mode")
		return
	}
	if !h.haStepDown() {
		writeError(w, http.StatusConflict, "this instance is not the leader; nothing to step down")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "stepping down"})
}
