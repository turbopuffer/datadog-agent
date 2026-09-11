# turbopuffer overlay build

This directory holds everything the fork adds outside the Go patch.

| File | Purpose |
|---|---|
| `Dockerfile` | Two-stage build. Compiles the agent inside the vendor image at the release tag, then lays one layer over the vendor image by digest. |
| `compile.sh` | Runs in the build stage. Embeds the config schema, builds base and patched binaries, runs the differential gate, strips the result. |
| `build.sh` | Host entry point. Exports the two source trees with `git archive`, builds, optionally pushes and prints the digest. |
| `tbot.yaml` | Teleport Machine ID config for the publish workflow. |
| `trivyignore` | CVE waivers, copied from the mirror pipeline. |
| `cosign.pub` | CI signing public key, copied from turbopuffer/turbopuffer. |

## Why the vendor image is the base

The fork changes one binary, `/opt/datadog-agent/bin/agent/agent`. Everything
else the container runs is Datadog's release image `datadoghq/agent` at the
same tag: the embedded Python and rtloader libraries the agent links against,
the s6 init scripts, the integrations under `conf.d`, the CA bundle, and the
trace, process, security and system-probe binaries, which stay stock.

`VENDOR_IMAGE` and `VENDOR_DIGEST` name that image. CI builds from the signed
mirror copy in ACR, a local build from the public registry. Compiling inside
that image, not in a generic Go image, makes the result link the runtime's own
glibc and rtloader. Publishing it as one layer on top of that image, not a
flattened copy, keeps every lower layer identical to Datadog's, which anyone
can confirm with `crane manifest` without trusting turbopuffer.

## Why not `dda inv agent.build`

`compile.sh` runs the same `go build ./cmd/agent` that upstream's `dda inv
agent.build` runs underneath, with the Go version, build tags and
`CGO_ENABLED` read from the vendor binary's buildinfo and the link flags from
`tasks/libs/common/utils.py`. The wrapper itself is not used because at this
tag it needs Bazel, its `schema.compress` step is `bazel run
//pkg/config/schema:install_compressed`, and its version helper recognises
only `-rc.N` and `-devel` suffixes, so a `7.82.2-tpuf.N` tag would not stamp.
The one Bazel product the binary needs, the compressed config schema, is
produced by `compile.sh` with the same two Python scripts and `zstd` call.

## Local build

```sh
git tag -a 7.82.2-tpuf.1 -m 7.82.2-tpuf.1
tpuf/build.sh
```

The default vendor image is the public `gcr.io/datadoghq/agent` at the pinned
digest. The default platform is `linux/amd64`, what the fleet runs. On an Apple
Silicon host set `PLATFORM=linux/arm64` to compile natively for a smoke test;
the gate derives build tags from the vendor binary of the platform it builds.
The build downloads Go and modules, so the first run takes a while. The gate
fails the build if the patched binary differs from the vendor binary in module
list or build tags, from the base build in dynamic section, or if symbols
outside the edited packages differ from a no-op build of the base sources.

## CI

Upstream runs the agent's test suite in GitLab CI. Its GitHub workflows are
PR bots, release automation and docs, and none of them can run outside
Datadog, so the fork branches carry none of them. Two upstream
`pull_request_target` bots, the CLA assistant and the community labeler, run
from the PR base branch regardless and are disabled at the repository level.
`.github/workflows/tpuf-ci.yml` runs on `tpuf-*` branches and their pull
requests: `go vet` and `go test -tags test` on the edited packages,
shellcheck on the build scripts, and the overlay build with its gate on amd64
against the public vendor image, followed by a check that the embedded schema
lists the new keys and that a malformed rule stops the agent.

## Publishing

`.github/workflows/publish-telemetry-images.yml` runs on a `7.*-tpuf.*` tag
push behind the `telemetry-publish` environment. It joins Teleport with
`tbot.yaml`, builds against the mirrored vendor image in ACR, pushes to
`turbopuffer.azurecr.io/telemetry/datadog-agent-tpuf`, scans, copies by digest
to ECR and GAR, and signs all three with the CI cosign key.

## Moving the base

1. Cut `tpuf-<version>` from the upstream tag and cherry-pick the fork commits.
2. Update `VENDOR_DIGEST` in `Dockerfile` and `build.sh` from `crane digest gcr.io/datadoghq/agent:<version>`.
3. Update the Go version and both tarball checksums in `Dockerfile` from the tag's `.go-version` and `https://go.dev/dl/?mode=json`.
4. From 7.84.0 the settings Go code is generated from the schema YAML. Run `tasks/schema/codegen_settings_main.py` before `go build` in `compile.sh`, the way `dda inv schema.codegen` does.
