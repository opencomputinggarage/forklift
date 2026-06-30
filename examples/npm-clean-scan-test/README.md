# npm Clean Scan Test

Small npm project for exercising forklift proxy caching and artifact scanning
with current, commonly used package versions.

This is the companion to `examples/npm-vulnerable-scan-test`. Use this project
when you want the normal/clean path: cache packages through forklift, scan them,
and verify that the UI mostly reports `clean` or low-noise results.

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
npm run cache:proxy
npm test
```

Then open forklift:

```text
http://127.0.0.1:8080
```

Go to the npm repository that has cached artifacts, usually `npmjs`, then open
the `Artifacts` tab. Use `Scan visible`, then click rows to inspect scan details.

`npm run cache:proxy` downloads packages through the local forklift npm proxy.
That cache fill is what creates artifacts for the scanner worker to process.
`npm test` prints the installed package versions, so it is a quick check that
the example resolved through the proxy correctly.

## Auth

This project commits registry configuration only. Do not commit auth tokens. If
your local repository requires a token, configure it outside the project:

```bash
npm config set //127.0.0.1:8080/npm/npm-public/:_authToken "$FORKLIFT_NPM_TOKEN"
```

If `npm run cache:proxy` returns `E401`, create or copy a repository token from
the forklift UI for `npm-public`, export it as `FORKLIFT_NPM_TOKEN`, then run the
config command above again.

Then rerun:

```bash
npm run cache:proxy
```
