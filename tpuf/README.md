# turbopuffer overlay build

This directory holds everything the fork adds outside the Go patch.

| File | Purpose |
|---|---|
| `Dockerfile` | Two-stage build. Compiles the agent inside the vendor image at the release tag, then lays one layer over the vendor image by digest. |
| `compile.sh` | Runs in the build stage. Builds base and patched binaries, runs the differential gate, strips the result. |
| `build.sh` | Host entry point. Exports the two source trees with `git archive`, builds, optionally pushes and prints the digest. |
| `tbot.yaml` | Teleport Machine ID config for the publish workflow. |
| `trivyignore` | CVE waivers, copied from the mirror pipeline. |
| `cosign.pub` | CI signing public key, copied from turbopuffer/turbopuffer. |

## Local build

```sh
git tag -a 7.82.2-tpuf.1 -m 7.82.2-tpuf.1
tpuf/build.sh
```

The default base is the public `gcr.io/datadoghq/agent` at the pinned digest.
The default platform is `linux/amd64`, what the fleet runs. On an Apple
Silicon host set `PLATFORM=linux/arm64` to compile natively for a smoke test;
the gate derives build tags from the vendor binary of the platform it builds.
The build downloads Go and modules, so the first run takes a while. The gate
fails the build if the patched binary differs from the vendor binary in module
list, build tags or dynamic section, or if symbols outside the edited packages
differ from a no-op build of the base sources.

## Publishing

`.github/workflows/publish-telemetry-images.yml` runs on a `7.*-tpuf.*` tag
push behind the `telemetry-publish` environment. It joins Teleport with
`tbot.yaml`, builds against the mirrored base in ACR, pushes to
`turbopuffer.azurecr.io/telemetry/datadog-agent`, scans, copies by digest to
ECR and GAR, and signs all three with the CI cosign key.

## Moving the base

1. Cut `tpuf-<version>` from the upstream tag and cherry-pick the fork commits.
2. Update `BASE_DIGEST` in `Dockerfile` and `build.sh` from `crane digest gcr.io/datadoghq/agent:<version>`.
3. Update the Go version and both tarball checksums in `Dockerfile` from the tag's `.go-version` and `https://go.dev/dl/?mode=json`.
4. From 7.84.0 the settings Go code is generated from the schema YAML. Add `dda inv schema.codegen` before `go build` in `compile.sh`.
