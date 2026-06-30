package artifactscan

// PolicyAction controls how a repository reacts to an artifact scan verdict.
type PolicyAction string

const (
	PolicyAudit PolicyAction = "audit"
	PolicyWarn  PolicyAction = "warn"
	PolicyBlock PolicyAction = "block"
)

// Policy is the repository-facing artifact scanning policy.
type Policy struct {
	Enabled        bool
	Action         PolicyAction
	Threshold      Severity
	BlockUnscanned bool
}

// Verdict is the request-time decision derived from stored scan state.
type Verdict struct {
	Allowed bool
	Action  PolicyAction
	Reason  string
}

// Evaluate returns the request-time verdict for a stored scan result. A nil
// result represents an unscanned artifact.
func Evaluate(policy Policy, result *Result) Verdict {
	if !policy.Enabled {
		return Verdict{Allowed: true, Action: PolicyAudit, Reason: "artifact scanning disabled"}
	}
	if policy.Action == "" {
		policy.Action = PolicyAudit
	}
	if policy.Threshold == "" {
		policy.Threshold = SeverityHigh
	}
	if result == nil {
		if policy.BlockUnscanned && policy.Action == PolicyBlock {
			return Verdict{Allowed: false, Action: PolicyBlock, Reason: "artifact has not been scanned"}
		}
		return Verdict{Allowed: true, Action: policy.Action, Reason: "artifact has not been scanned"}
	}
	if result.Status != StatusCompleted {
		return Verdict{Allowed: true, Action: policy.Action, Reason: string(result.Status)}
	}
	if SeverityRank(result.MaxSeverity) < SeverityRank(policy.Threshold) {
		return Verdict{Allowed: true, Action: policy.Action, Reason: "below threshold"}
	}
	if policy.Action == PolicyBlock {
		return Verdict{Allowed: false, Action: policy.Action, Reason: "artifact scan policy violation"}
	}
	return Verdict{Allowed: true, Action: policy.Action, Reason: "artifact scan policy violation"}
}
