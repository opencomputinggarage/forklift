# Artifact Scanning Design Notes

## Status

Draft. This document records the current design direction for optional
artifact-level security scanning in forklift. It is intentionally not an
implementation plan yet.

## Context

forklift already has supply-chain controls that work at package-coordinate
level:

- OSV-based vulnerability policy
- deps.dev-based license policy
- age policy for fresh upstream versions
- package approval and version deny decisions

These controls are lightweight and fit forklift's core shape: a single Go
binary with embedded SQLite metadata, content-addressed blob storage, and
Kubernetes-native deployment.

Artifact Keeper shows a broader model: scanner orchestration, Trivy, Grype,
OpenSCAP, SBOM storage, Dependency-Track integration, scan policies, and
quarantine workflows. That model is powerful, but its own audit notes identify
SBOM/scanner integration as one of the largest defect areas. The lesson for
forklift is not "copy the stack"; it is to keep the integration boundary small,
make scanner state explicit, and add scanning only as an optional profile.
Its public scanning guide also highlights operational concerns that matter even
for a smaller implementation: scanner database freshness, scheduled rescans,
manual rescans, hash-based deduplication, scanner availability, and scan
performance metrics.

Harbor is also already used for Docker/OCI images in the target environment, so
forklift should not duplicate Harbor's image registry and image scanning role.

## Goals

- Add a path for artifact-level vulnerability scanning of Maven, npm, PyPI,
  Cargo, and Go artifacts.
- Keep scanning optional and disabled by default.
- Keep forklift's serving path fast: request handling must consult stored scan
  verdicts only, never run a scanner inline.
- Reuse scan results by blob SHA-256 so identical bytes are scanned once.
- Allow scanner workers to scale independently from the registry process.
- Preserve future extension points for SBOM storage and Dependency-Track export.
- Track scanner database freshness so operators can distinguish "clean with a
  current database" from "clean with stale vulnerability data."
- Support explicit and scheduled rescans without making them part of the first
  serving-path implementation.

## Non-Goals

- Do not make forklift an OCI image scanner. Harbor owns that surface.
- Do not embed scanner binaries in the forklift server image.
- Do not require Syft, Grype, Trivy, OpenSCAP, or Dependency-Track for normal
  operation.
- Do not add OpenSCAP as a default scanner. It is mainly useful for operating
  system, RPM/DEB, container image, and compliance profiles, not ordinary
  language package artifacts.
- Do not block unscanned artifacts by default during the initial rollout.
- Do not build a full SBOM management platform inside forklift.

## Proposed Architecture

Scanning should be split into two processes:

```text
forklift server
  - stores artifacts as content-addressed blobs
  - creates scan jobs
  - stores scan results and policy verdicts
  - serves package traffic
  - checks stored verdicts in policy gates

scanner worker
  - claims scan jobs
  - downloads or opens blobs by SHA-256
  - runs external scanner tools
  - normalizes results
  - posts results back to forklift
```

The initial scanner worker should be separate from the server binary. In
Kubernetes this can be a small Deployment enabled by the Helm chart:

```yaml
artifactScanning:
  enabled: false
  worker:
    replicas: 1
  policy:
    action: audit
    threshold: high
    blockUnscanned: false
```

The worker security model, including disposable Job execution, short-lived scan
tokens, NetworkPolicy, and optional gVisor/Kata runtime isolation, is described
separately in [Artifact Scanner Worker Security Design](artifact-scanner-worker-security.md).

## Scanner Selection

The first useful scanner to evaluate is Grype.

Rationale:

- It can scan directories, archives, filesystems, SBOMs, and container
  references.
- It is directly relevant to package artifacts.
- It is lighter than adopting a full Trivy + Grype + OpenSCAP + Dependency-Track
  stack.
- It can be introduced without committing to SBOM persistence.

Syft is a later extension when SBOMs are a product requirement:

```text
artifact -> Syft -> SBOM stored by blob SHA-256
SBOM     -> Grype -> vulnerability findings
```

Dependency-Track is a later external integration, not a core dependency:

```text
stored SBOM -> Dependency-Track upload -> long-term portfolio tracking
```

Grant can be considered only if license enforcement needs to move from
deps.dev coordinate checks to SBOM-based license compliance. It is not part of
the first design slice.

## Data Model Direction

The model should separate jobs, scan summaries, findings, and optional SBOMs.

```text
artifact_scan_jobs
  id
  blob_sha256
  scanner
  status              queued | running | completed | failed | not_applicable
  attempts
  error
  created_at
  started_at
  finished_at

artifact_scan_results
  id
  blob_sha256
  scanner
  scanner_version
  database_version
  database_updated_at
  status              completed | failed | not_applicable | reused
  max_severity
  finding_count
  critical_count
  high_count
  medium_count
  low_count
  scanned_at
  source_result_id    nullable, for reused results

artifact_scan_findings
  id
  result_id
  vuln_id
  severity
  package_name
  package_version
  fixed_version
  source
  source_url

artifact_sboms
  id
  blob_sha256
  format              cyclonedx | spdx
  generator
  generator_version
  content_json
  created_at
```

The `not_applicable` state is important. A scanner that does not apply to an
artifact must not produce a clean result. It also should not be treated as a
scanner crash. Those are different operational states.

Findings and SBOM inventory should not be conflated. A vulnerability finding is
evidence of a problem. An SBOM component inventory is evidence of what was
observed, including packages with no known vulnerabilities.

## Deduplication

Scan deduplication should be based on blob identity:

```text
blob_sha256 + scanner + scanner_version + scanner_policy_version
```

The initial version can omit `scanner_policy_version` if policy is applied at
read time rather than embedded into scanner output. If scanner configuration
affects the output, it must become part of the dedup key.

Deduplication should support two paths:

- Reuse a completed result for identical bytes.
- Force a rescan when an operator suspects the previous scan was incomplete,
  stale, or produced with a broken scanner version.

Clean results are only meaningful relative to scanner database freshness. A
future implementation should invalidate or age out reusable scan results when
the scanner database changes enough to make a rescan valuable. The first slice
can expose database freshness as metadata and leave automatic invalidation for a
later phase.

## Policy Model

Repository policy should start audit-only:

```yaml
artifactScanning:
  enabled: true
  policy:
    action: audit       # audit | warn | block
    threshold: high     # critical | high | medium | low
    blockUnscanned: false
```

Serving behavior:

- If scanning is disabled, serve normally.
- If no verdict exists and `blockUnscanned=false`, serve and enqueue/keep the
  job.
- If no verdict exists and `blockUnscanned=true`, block only for repositories
  that explicitly opt into that strict posture.
- If the latest applicable verdict exceeds the threshold, apply `audit`, `warn`,
  or `block`.
- `failed` and `not_applicable` must be configurable separately. A scanner crash
  is not the same as a scanner not applying to a package.

## Trigger Points

Scan jobs can be created after:

- hosted upload
- proxy cache write
- explicit admin rescan request
- scheduled rescan for stale or high-value artifacts

The initial implementation should not scan during a client download request.
For proxy repositories, cache-write-time enqueue is enough for the first slice.

Scheduled rescan should be bounded. Useful knobs:

- maximum artifact age to rescan
- maximum jobs per interval
- low-priority queue mode
- force rescan vs reuse-if-fresh

## Artifact Keeper Lessons To Preserve

The Artifact Keeper design and audit history point to several guardrails:

- Scanner applicability must be decided before recording a clean result.
- Failed scanner execution, non-applicable scanner, and clean scan are distinct
  states.
- Scan result reuse needs concurrency protection. Duplicate running scans for
  the same artifact and scanner are easy to create without an atomic placeholder
  step.
- Inventory/SBOM persistence can fail after findings were persisted. That needs
  a visible degraded state if SBOMs are introduced.
- Dependency-Track should remain optional because it is operationally heavy.
- OpenSCAP should remain specialized, not a default package scanner.
- Scanner database freshness and scan cache freshness are separate concepts.
  Both need visibility before enforcement becomes trustworthy.

## Rollout Plan

### Phase 0: Design and API Shape

- Add no scanner binaries.
- Define the internal state model and API contracts.
- Decide whether scan results are stored by `blob_sha256` only or also linked
  directly to each artifact row for faster UI queries.
- Define metrics and audit events.
- Define scanner database freshness fields and API shape.

### Phase 1: Grype Audit-Only Worker

- Add an optional worker that runs Grype against stored blobs.
- Store normalized vulnerability findings by `blob_sha256`.
- Show scan status and max severity in the API/UI.
- Keep repository policy in `audit`.
- Keep `blockUnscanned=false`.

### Phase 2: Policy Enforcement

- Add `warn` and `block` modes.
- Add manual rescan.
- Add stale-result handling.
- Add per-repository threshold configuration.
- Add basic scanner availability and database freshness checks.

### Phase 3: Optional SBOM

- Add Syft-based SBOM generation only if SBOM storage or export is required.
- Store SBOMs by `blob_sha256`.
- Keep findings and SBOM component inventory separate.
- Add scheduled rescan once clean-result freshness is visible and tested.

### Phase 4: External Integrations

- Add Dependency-Track export for stored SBOMs.
- Add Grant only if license compliance needs SBOM-level evaluation.

## Open Questions

- Should scan jobs live in SQLite, or should the worker poll an API that hides
  the queue implementation?
- Should the worker read blobs through the public package API, an internal API,
  or direct storage access?
- How long should a clean scan remain reusable before requiring a rescan?
- Should scanner database freshness be represented in the verdict?
- How should group repositories present scan status when the same blob is
  reachable through several repository names?
- What is the smallest UI surface: status badge only, or a findings table in
  the artifact detail view?
- Should manual and bulk scan APIs return job IDs, scan result IDs, or both?
- Should scheduled rescans be repository-scoped, global, or driven by scan
  result age and severity?

## Current Recommendation

Do not add the full scanner stack now.

Build the extension point first, then evaluate a Grype-only audit worker. Add
Syft only when SBOM storage or Dependency-Track export becomes a real product
requirement. Keep Harbor responsible for container images, and keep forklift
focused on language package repositories and policy gates.
