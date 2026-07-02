import type { ArtifactScanFinding } from "@/api";

export const severityOrder = ["critical", "high", "medium", "low", "negligible", "unknown"];

export const severityRank = severityOrder.reduce(
  (acc, severity, idx) => ({ ...acc, [severity]: severityOrder.length - idx }),
  {} as Record<string, number>,
);

export function normalizedSeverity(severity?: string) {
  return (severity || "unknown").toLowerCase();
}

export function sortedFindings(findings: ArtifactScanFinding[]) {
  return [...findings].sort((a, b) => {
    const rank = (severityRank[normalizedSeverity(b.severity)] ?? 0) - (severityRank[normalizedSeverity(a.severity)] ?? 0);
    if (rank !== 0) return rank;
    return `${a.package_name}:${a.vulnerability_id}`.localeCompare(`${b.package_name}:${b.vulnerability_id}`);
  });
}

export function summarizeFindings(findings: ArtifactScanFinding[]) {
  return findings.reduce((acc, finding) => {
    const severity = normalizedSeverity(finding.severity);
    acc[severity] = (acc[severity] ?? 0) + 1;
    return acc;
  }, {} as Record<string, number>);
}

export function packageSummary(findings: ArtifactScanFinding[]) {
  const byPackage = new Map<string, { name: string; count: number; highest: string }>();
  for (const finding of findings) {
    const name = finding.package_name || "unknown";
    const severity = normalizedSeverity(finding.severity);
    const existing = byPackage.get(name);
    if (!existing) {
      byPackage.set(name, { name, count: 1, highest: severity });
      continue;
    }
    existing.count += 1;
    if ((severityRank[severity] ?? 0) > (severityRank[existing.highest] ?? 0)) existing.highest = severity;
  }
  return [...byPackage.values()].sort((a, b) => {
    const severityDelta = (severityRank[b.highest] ?? 0) - (severityRank[a.highest] ?? 0);
    if (severityDelta !== 0) return severityDelta;
    return b.count - a.count;
  });
}

export function filterFindings(findings: ArtifactScanFinding[], severity: string, query: string) {
  const q = query.trim().toLowerCase();
  return findings.filter((finding) => {
    if (severity && normalizedSeverity(finding.severity) !== severity) return false;
    if (!q) return true;
    return [
      finding.vulnerability_id,
      finding.package_name,
      finding.package_version,
      finding.package_type,
      finding.package_purl,
      finding.source,
      finding.match_type,
      ...(finding.fixed_versions ?? []),
    ].filter(Boolean).join(" ").toLowerCase().includes(q);
  });
}
