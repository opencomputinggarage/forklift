package repo

import (
	"errors"
	"net/http"
	"strings"

	"github.com/younsl/o/box/kubernetes/forklift/internal/audit"
	"github.com/younsl/o/box/kubernetes/forklift/internal/auth"
	"github.com/younsl/o/box/kubernetes/forklift/internal/meta"
	"github.com/younsl/o/box/kubernetes/forklift/internal/repoconfig"
)

// licenseGate enforces the per-repository license policy for proxy reads. It
// consults stored resolution results only (never blocking on a live lookup): a
// not-yet-resolved coordinate is queued for async resolution and, unless
// BlockUnresolved is set, served meanwhile. A resolved coordinate whose
// licenses violate the policy (a denied license, or a license outside a
// non-empty allow list) is blocked, warned, or audited per Action.
//
// Returns true when the request was blocked (response written).
func (m *Manager) licenseGate(w http.ResponseWriter, r *http.Request, res resolved, pkg, version string) bool {
	if m.resolver == nil || res.repo.Type != meta.TypeProxy {
		return false
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		return false
	}
	cfg := res.cfg.License
	if !cfg.Enabled || pkg == "" || version == "" {
		return false
	}
	system := depsDevSystem(res.repo.Format)
	if system == "" {
		return false
	}

	scan, err := m.store.GetLicenseScan(r.Context(), system, pkg, version)
	if errors.Is(err, meta.ErrNotFound) {
		m.enqueueResolve(system, pkg, version)
		// Unknown coordinate: fail open unless the policy opts into blocking
		// pending resolutions under an enforcing posture.
		if cfg.EffectiveAction() == repoconfig.VulnActionBlock && cfg.BlockUnresolved {
			http.Error(w, "package pending license resolution: "+pkg, http.StatusForbidden)
			return true
		}
		return false
	}
	if err != nil {
		// Best-effort: a lookup error must not break serving.
		m.engine.log.Error("license lookup failed", "repo", res.repo.Name, "package", pkg, "version", version, "err", err)
		return false
	}

	violated, reason := licenseViolation(scan.Licenses, cfg.Deny, cfg.Allow)
	if !violated {
		return false
	}

	action := cfg.EffectiveAction()
	m.licenseBlocked.WithLabelValues(res.repo.Name, action).Inc()
	licenses := strings.Join(scan.Licenses, ",")
	if action == repoconfig.VulnActionAudit || action == repoconfig.VulnActionWarn {
		m.engine.log.Warn("license policy: would block",
			"repo", res.repo.Name, "package", pkg, "version", version,
			"licenses", licenses, "reason", reason, "action", action)
		return false
	}

	if m.rec != nil {
		var username string
		if p := auth.FromContext(r.Context()); p != nil {
			username = p.Username
		}
		m.rec.Record(audit.Event{
			Repo:      res.repo.Name,
			Action:    meta.EventLicenseBlock,
			Path:      pkg + "@" + version,
			Username:  username,
			Method:    r.Method,
			Status:    http.StatusForbidden,
			ClientIP:  audit.ClientIP(r),
			UserAgent: r.UserAgent(),
		})
	}
	m.engine.log.Warn("package blocked by license policy",
		"repo", res.repo.Name, "package", pkg, "version", version,
		"licenses", licenses, "reason", reason)
	http.Error(w, "blocked: license policy ("+reason+") for "+pkg+"@"+version, http.StatusForbidden)
	return true
}

// licenseViolation reports whether a coordinate's licenses violate the policy
// and a human-readable reason. A license in deny always violates; if allow is
// non-empty, any license outside allow violates (allow-list mode). Coordinates
// with no resolved license never violate here (the BlockUnresolved path governs
// the unknown case). Matching is case-insensitive on the SPDX identifier.
func licenseViolation(licenses, deny, allow []string) (bool, string) {
	denySet := lowerSet(deny)
	for _, l := range licenses {
		if denySet[strings.ToLower(l)] {
			return true, "denied license " + l
		}
	}
	if len(allow) > 0 && len(licenses) > 0 {
		allowSet := lowerSet(allow)
		for _, l := range licenses {
			if !allowSet[strings.ToLower(l)] {
				return true, "license " + l + " not in allow list"
			}
		}
	}
	return false, ""
}

func lowerSet(items []string) map[string]bool {
	set := make(map[string]bool, len(items))
	for _, it := range items {
		it = strings.TrimSpace(it)
		if it != "" {
			set[strings.ToLower(it)] = true
		}
	}
	return set
}
