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
	ScannerProfile string
	Action         PolicyAction
	Threshold      Severity
	BlockUnscanned bool
}

// RequestDecision is the request-time decision derived from a stored verdict.
type RequestDecision struct {
	Allowed bool
	Action  PolicyAction
	Reason  string
}

// ComputeVerdict applies repository policy to a scan report. A nil report
// represents an artifact with no usable report yet.
func ComputeVerdict(repositoryID int64, blobSHA256 string, policy Policy, result *Result, policyHash string) Verdict {
	if policy.ScannerProfile == "" {
		policy.ScannerProfile = "grype-default"
	}
	if policy.Action == "" {
		policy.Action = PolicyAudit
	}
	if policy.Threshold == "" {
		policy.Threshold = SeverityHigh
	}
	v := Verdict{
		RepositoryID:   repositoryID,
		BlobSHA256:     blobSHA256,
		ScannerProfile: policy.ScannerProfile,
		PolicyHash:     policyHash,
		Status:         VerdictPending,
		Reason:         "artifact has not been scanned",
		MaxSeverity:    SeverityUnknown,
	}
	if !policy.Enabled {
		v.Status = VerdictAllow
		v.Reason = "artifact scanning disabled"
		return v
	}
	if result == nil {
		if policy.BlockUnscanned && policy.Action == PolicyBlock {
			v.Status = VerdictBlock
		}
		return v
	}
	v.ResultID = result.ID
	v.MaxSeverity = result.MaxSeverity
	switch result.Status {
	case ReportCompleted:
		if SeverityRank(result.MaxSeverity) < SeverityRank(policy.Threshold) {
			v.Status = VerdictAllow
			v.Reason = "below threshold"
			return v
		}
		v.Reason = "artifact scan policy violation"
		switch policy.Action {
		case PolicyBlock:
			v.Status = VerdictBlock
		case PolicyWarn:
			v.Status = VerdictWarn
		default:
			v.Status = VerdictAudit
		}
	case ReportFailed:
		v.Status = VerdictPending
		v.Reason = "scanner failed"
	case ReportNotApplicable:
		v.Status = VerdictAllow
		v.Reason = "scanner not applicable"
	case ReportSkippedTooLarge:
		v.Status = VerdictPending
		v.Reason = "artifact too large to scan"
	case ReportReused:
		v.Status = VerdictAllow
		v.Reason = "reused scan report"
	default:
		v.Status = VerdictPending
		v.Reason = string(result.Status)
	}
	return v
}

// Decide returns the serving decision for a stored verdict.
func Decide(policy Policy, verdict *Verdict) RequestDecision {
	if !policy.Enabled {
		return RequestDecision{Allowed: true, Action: PolicyAudit, Reason: "artifact scanning disabled"}
	}
	if policy.Action == "" {
		policy.Action = PolicyAudit
	}
	if verdict == nil {
		if policy.BlockUnscanned && policy.Action == PolicyBlock {
			return RequestDecision{Allowed: false, Action: PolicyBlock, Reason: "artifact has not been scanned"}
		}
		return RequestDecision{Allowed: true, Action: policy.Action, Reason: "artifact has not been scanned"}
	}
	switch verdict.Status {
	case VerdictBlock:
		return RequestDecision{Allowed: false, Action: PolicyBlock, Reason: verdict.Reason}
	default:
		return RequestDecision{Allowed: true, Action: policy.Action, Reason: verdict.Reason}
	}
}
