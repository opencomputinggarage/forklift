import { describe, expect, test } from "vitest";

import type { ArtifactScanFinding } from "@/api";
import { filterFindings, packageSummary, sortedFindings, summarizeFindings } from "@/routes/workspace/repositories/$id/-artifact-scan-findings";

const findings: ArtifactScanFinding[] = [
  {
    vulnerability_id: "GHSA-medium",
    severity: "medium",
    package_name: "next",
    package_version: "12.0.0",
    package_purl: "pkg:npm/next@12.0.0",
    fixed_versions: ["12.3.4"],
    source: "ghsa",
  },
  {
    vulnerability_id: "CVE-critical",
    severity: "critical",
    package_name: "axios",
    package_version: "0.21.1",
    package_purl: "pkg:npm/axios@0.21.1",
    fixed_versions: ["0.21.4"],
    source: "nvd",
  },
  {
    vulnerability_id: "GHSA-high",
    severity: "high",
    package_name: "axios",
    package_version: "0.21.1",
    package_purl: "pkg:npm/axios@0.21.1",
    fixed_versions: ["0.21.2"],
    source: "ghsa",
  },
];

describe("artifact scan finding helpers", () => {
  test("sorts findings by severity before package name", () => {
    expect(sortedFindings(findings).map((finding) => finding.vulnerability_id)).toEqual([
      "CVE-critical",
      "GHSA-high",
      "GHSA-medium",
    ]);
  });

  test("summarizes severities and top packages", () => {
    expect(summarizeFindings(findings)).toMatchObject({
      critical: 1,
      high: 1,
      medium: 1,
    });
    expect(packageSummary(findings)).toEqual([
      { name: "axios", count: 2, highest: "critical" },
      { name: "next", count: 1, highest: "medium" },
    ]);
  });

  test("filters by severity and searchable vulnerability metadata", () => {
    expect(filterFindings(findings, "high", "").map((finding) => finding.vulnerability_id)).toEqual(["GHSA-high"]);
    expect(filterFindings(findings, "", "0.21.4").map((finding) => finding.vulnerability_id)).toEqual(["CVE-critical"]);
    expect(filterFindings(findings, "", "pkg:npm/next").map((finding) => finding.vulnerability_id)).toEqual(["GHSA-medium"]);
  });
});
