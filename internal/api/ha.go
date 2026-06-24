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
}

// SetHAStatus injects the provider that assembles live HA status. Wired from
// main once leader election and the storage backend are known.
func (h *Handler) SetHAStatus(fn func(context.Context) HAStatus) { h.haStatus = fn }

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
