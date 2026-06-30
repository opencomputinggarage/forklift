# npm Vulnerable Scan Test

Small npm project for exercising forklift proxy caching and artifact scanning
with intentionally old dependency versions.

## Packages

- `axios@0.21.1`: small, quick scan target.
- `next@12.0.0`: pulls more transitive packages, useful for large artifact list
  and batch scan UI testing.

## Run

Start forklift with artifact scanning enabled:

```bash
make scan-dev
```

In another terminal, keep a worker polling:

```bash
make artifact-scan-worker-loop
```

From this directory:

```bash
npm install --cache ./.npm-cache --prefer-online
```

Then open forklift:

```text
http://127.0.0.1:8080
```

Go to the npm repository that has cached artifacts, usually `npmjs`, then open
the `Artifacts` tab. Use `Scan visible` or click a row to inspect scan details.

## Auth

This project commits registry configuration only. Do not commit auth tokens. If
your local repository requires a token, configure it outside the project:

```bash
npm config set //127.0.0.1:8080/npm/npm-public/:_authToken "$FORKLIFT_NPM_TOKEN"
```
