{{/* Expand the name of the chart. */}}
{{- define "forklift.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/* Fully qualified app name. */}}
{{- define "forklift.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Fully qualified container image reference, joining registry, repository and tag.
The registry is optional: when empty the repository is used as-is (so it may
itself carry a host). Tag defaults to the chart appVersion.
*/}}
{{- define "forklift.image" -}}
{{- $tag := .Values.image.tag | default .Chart.AppVersion -}}
{{- if .Values.image.registry -}}
{{- printf "%s/%s:%s" .Values.image.registry .Values.image.repository $tag -}}
{{- else -}}
{{- printf "%s:%s" .Values.image.repository $tag -}}
{{- end -}}
{{- end }}

{{- define "forklift.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "forklift.labels" -}}
helm.sh/chart: {{ include "forklift.chart" . }}
{{ include "forklift.selectorLabels" . }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{- define "forklift.selectorLabels" -}}
app.kubernetes.io/name: {{ include "forklift.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{- define "forklift.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "forklift.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
s3Enabled is the effective S3 toggle: the operator explicitly set the s3 backend,
OR the bundled MinIO subchart is enabled (which is wired to the s3 backend
automatically). Used by every template that branches on object-storage mode so
"minio.enabled" alone is enough to switch the whole chart to S3.
*/}}
{{- define "forklift.s3Enabled" -}}
{{- if or (eq .Values.storage.backend "s3") .Values.minio.enabled -}}true{{- else -}}false{{- end -}}
{{- end -}}

{{/*
Fully qualified name of the bundled MinIO release, matching the minio subchart's
own fullname logic (release-name + chart-name, unless the release name already
contains it). Honors minio.fullnameOverride.
*/}}
{{- define "forklift.minioFullname" -}}
{{- if .Values.minio.fullnameOverride -}}
{{- .Values.minio.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else if contains "minio" .Release.Name -}}
{{- .Release.Name | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-minio" .Release.Name | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}

{{/*
Effective S3 endpoint: an explicitly configured endpoint wins; otherwise, when
MinIO is bundled, point at the in-cluster MinIO API service. Empty (AWS default).
*/}}
{{- define "forklift.s3Endpoint" -}}
{{- if .Values.storage.s3.endpoint -}}
{{- .Values.storage.s3.endpoint -}}
{{- else if .Values.minio.enabled -}}
{{- printf "http://%s:9000" (include "forklift.minioFullname" .) -}}
{{- end -}}
{{- end -}}

{{/*
Effective S3 bucket: an explicit storage.s3.bucket wins; otherwise the first
bucket the MinIO subchart provisions. Fails when neither is available.
*/}}
{{- define "forklift.s3Bucket" -}}
{{- if .Values.storage.s3.bucket -}}
{{- .Values.storage.s3.bucket -}}
{{- else if and .Values.minio.enabled .Values.minio.buckets -}}
{{- (first .Values.minio.buckets).name -}}
{{- else -}}
{{- fail "storage.s3.bucket is required when storage.backend is s3 (or define minio.buckets when using the bundled MinIO)" -}}
{{- end -}}
{{- end -}}

{{/*
Validate mutually exclusive storage/HA modes. The s3 backend (including the
bundled MinIO) already shares blobs and snapshots metadata to S3, so PV-based
peer replication is redundant and would compete for the metadata snapshot.
*/}}
{{- define "forklift.validateStorage" -}}
{{- if and (eq (include "forklift.s3Enabled" .) "true") .Values.replication.enabled -}}
{{- fail "object-storage mode (storage.backend=s3 or minio.enabled=true) is incompatible with replication.enabled=true; disable one (s3 mode shares blobs and snapshots metadata to S3, so peer replication is unnecessary)" -}}
{{- end -}}
{{- if not (has .Values.storage.backend (list "fs" "s3")) -}}
{{- fail (printf "storage.backend must be \"fs\" or \"s3\", got %q" .Values.storage.backend) -}}
{{- end -}}
{{- end -}}

{{/* haEnabled resolves the HA toggle: explicit value, else replicaCount > 1. */}}
{{- define "forklift.haEnabled" -}}
{{- if kindIs "bool" .Values.ha.enabled }}
{{- .Values.ha.enabled }}
{{- else }}
{{- gt (int .Values.replicaCount) 1 }}
{{- end }}
{{- end }}

{{- define "forklift.leaseName" -}}
{{- default (printf "%s-leader" (include "forklift.fullname" .)) .Values.ha.leaseName }}
{{- end }}

{{- define "forklift.headlessServiceName" -}}
{{ include "forklift.fullname" . }}-headless
{{- end }}

{{/*
Container environment shared by the Deployment (shared RWX volume mode) and the
StatefulSet (PV-based replication mode).
*/}}
{{- define "forklift.env" -}}
- name: POD_NAME
  valueFrom:
    fieldRef:
      fieldPath: metadata.name
- name: POD_NAMESPACE
  valueFrom:
    fieldRef:
      fieldPath: metadata.namespace
- name: FORKLIFT_DATA_DIR
  value: /data
{{- if eq (include "forklift.s3Enabled" .) "true" }}
- name: FORKLIFT_STORAGE_BACKEND
  value: "s3"
- name: FORKLIFT_STORAGE_S3_BUCKET
  value: {{ include "forklift.s3Bucket" . | quote }}
{{- with .Values.storage.s3.prefix }}
- name: FORKLIFT_STORAGE_S3_PREFIX
  value: {{ . | quote }}
{{- end }}
{{- $region := .Values.storage.s3.region }}
{{- if and (not $region) .Values.minio.enabled }}{{ $region = "us-east-1" }}{{- end }}
{{- with $region }}
- name: FORKLIFT_STORAGE_S3_REGION
  value: {{ . | quote }}
{{- end }}
{{- with (include "forklift.s3Endpoint" .) }}
- name: FORKLIFT_STORAGE_S3_ENDPOINT
  value: {{ . | quote }}
{{- end }}
- name: FORKLIFT_STORAGE_S3_FORCE_PATH_STYLE
  value: {{ or .Values.storage.s3.forcePathStyle .Values.minio.enabled | quote }}
- name: FORKLIFT_STORAGE_META_SYNC_INTERVAL
  value: {{ .Values.storage.s3.metaSyncInterval | quote }}
{{- /*
S3 credentials. An explicit existingSecret wins. Otherwise, when MinIO is
bundled, read the root credentials forklift mirrors into its own Secret. With
neither, no static keys are set and the app falls back to the AWS default
credential chain (EKS IRSA / Pod Identity).
*/}}
{{- $credSecret := "" }}
{{- if .Values.storage.s3.existingSecret }}{{ $credSecret = .Values.storage.s3.existingSecret }}
{{- else if .Values.minio.enabled }}{{ $credSecret = include "forklift.fullname" . }}{{- end }}
{{- with $credSecret }}
- name: FORKLIFT_STORAGE_S3_ACCESS_KEY_ID
  valueFrom:
    secretKeyRef:
      name: {{ . }}
      key: access-key-id
- name: FORKLIFT_STORAGE_S3_SECRET_ACCESS_KEY
  valueFrom:
    secretKeyRef:
      name: {{ . }}
      key: secret-access-key
{{- end }}
{{- end }}
- name: FORKLIFT_LOG_LEVEL
  value: {{ .Values.log.level | quote }}
- name: FORKLIFT_LOG_FORMAT
  value: {{ .Values.log.format | quote }}
- name: FORKLIFT_ANONYMOUS_READ
  value: {{ .Values.auth.anonymousRead | quote }}
- name: FORKLIFT_SEED_DEFAULT_REPOS
  value: {{ .Values.seedDefaultRepos | quote }}
- name: FORKLIFT_AUDIT_ENABLED
  value: {{ .Values.audit.enabled | quote }}
- name: FORKLIFT_AUDIT_RETENTION
  value: {{ .Values.audit.retention | quote }}
- name: FORKLIFT_SESSION_TTL
  value: {{ .Values.auth.sessionTTL | quote }}
{{- if eq (include "forklift.haEnabled" .) "true" }}
- name: FORKLIFT_HA_ENABLED
  value: "true"
- name: FORKLIFT_HA_LEASE_NAME
  value: {{ include "forklift.leaseName" . }}
- name: FORKLIFT_HA_LEASE_DURATION
  value: {{ .Values.ha.leaseDuration | quote }}
- name: FORKLIFT_HA_RENEW_DEADLINE
  value: {{ .Values.ha.renewDeadline | quote }}
- name: FORKLIFT_HA_RETRY_PERIOD
  value: {{ .Values.ha.retryPeriod | quote }}
{{- end }}
{{- if .Values.replication.enabled }}
- name: FORKLIFT_REPLICATION_ENABLED
  value: "true"
- name: FORKLIFT_REPLICATION_PEER_SERVICE
  value: "{{ include "forklift.headlessServiceName" . }}.{{ .Release.Namespace }}.svc.cluster.local"
- name: FORKLIFT_REPLICATION_INTERVAL
  value: {{ .Values.replication.interval | quote }}
- name: FORKLIFT_REPLICATION_TOKEN
  valueFrom:
    secretKeyRef:
      name: {{ include "forklift.fullname" . }}
      key: replication-token
{{- end }}
{{- if .Values.auth.oidc.enabled }}
- name: FORKLIFT_OIDC_ENABLED
  value: "true"
- name: FORKLIFT_OIDC_ISSUER_URL
  value: {{ .Values.auth.oidc.issuerURL | quote }}
- name: FORKLIFT_OIDC_CLIENT_ID
  value: {{ .Values.auth.oidc.clientID | quote }}
- name: FORKLIFT_OIDC_REDIRECT_URL
  value: {{ .Values.auth.oidc.redirectURL | quote }}
- name: FORKLIFT_OIDC_USERNAME_CLAIM
  value: {{ .Values.auth.oidc.usernameClaim | quote }}
- name: FORKLIFT_OIDC_GROUPS_CLAIM
  value: {{ .Values.auth.oidc.groupsClaim | quote }}
- name: FORKLIFT_OIDC_CLIENT_SECRET
  valueFrom:
    secretKeyRef:
      name: {{ include "forklift.fullname" . }}
      key: oidc-client-secret
{{- end }}
- name: FORKLIFT_SESSION_SECRET
  valueFrom:
    secretKeyRef:
      name: {{ include "forklift.fullname" . }}
      key: session-secret
- name: FORKLIFT_BOOTSTRAP_ADMIN_USER
  value: {{ .Values.auth.bootstrap.adminUser | quote }}
- name: FORKLIFT_BOOTSTRAP_ADMIN_PASSWORD
  valueFrom:
    secretKeyRef:
      name: {{ include "forklift.fullname" . }}
      key: bootstrap-admin-password
{{- if .Values.auth.rbac.enabled }}
- name: FORKLIFT_RBAC_POLICY_FILE
  value: /etc/forklift/rbac/policy.csv
- name: FORKLIFT_RBAC_DEFAULT_ROLE
  value: {{ .Values.auth.rbac.policyDefault | quote }}
{{- if .Values.auth.rbac.accounts }}
- name: FORKLIFT_RBAC_ACCOUNTS_DIR
  value: /etc/forklift/accounts
{{- end }}
{{- end }}
- name: FORKLIFT_OSV_URL
  value: {{ .Values.vuln.osvUrl | quote }}
{{- with .Values.externalUrl }}
- name: FORKLIFT_EXTERNAL_URL
  value: {{ . | quote }}
{{- end }}
{{- with .Values.extraEnv }}
{{ toYaml . }}
{{- end }}
{{- end }}

{{/*
RBAC volume mounts: the policy.csv ConfigMap and, when local accounts are
declared, the per-account password Secret projected as files named by username.
*/}}
{{- define "forklift.rbacVolumeMounts" -}}
{{- if .Values.auth.rbac.enabled }}
- name: rbac-policy
  mountPath: /etc/forklift/rbac
  readOnly: true
{{- if .Values.auth.rbac.accounts }}
- name: rbac-accounts
  mountPath: /etc/forklift/accounts
  readOnly: true
{{- end }}
{{- end }}
{{- end }}

{{- define "forklift.rbacVolumes" -}}
{{- if .Values.auth.rbac.enabled }}
- name: rbac-policy
  configMap:
    name: {{ include "forklift.fullname" . }}-rbac
{{- if .Values.auth.rbac.accounts }}
- name: rbac-accounts
  secret:
    secretName: {{ include "forklift.fullname" . }}
    items:
      {{- range .Values.auth.rbac.accounts }}
      - key: {{ printf "local-user-%s-password" .name }}
        path: {{ .name }}
      {{- end }}
{{- end }}
{{- end }}
{{- end }}
