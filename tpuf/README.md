# turbopuffer overlay build

This directory holds additional tpuf specific files needed to build/publish the agent fork.

## Local build

```sh
git tag -a 7.82.2-tpuf.1 -m 7.82.2-tpuf.1
tpuf/build.sh
```

The default vendor image is the public `gcr.io/datadoghq/agent` at the pinned
digest. The default platform is `linux/amd64`, which is what we run. On an Apple
Silicon host set `PLATFORM=linux/arm64` to compile natively.

Some additional info:
- Build will install Go and modules, so no need to have Go installed locally first
- The Build will fail under the following gates:
  - The patched binary must use the same Go modules and build tags as the Datadog binary
  - The patched binary must link the same shared libs as the base build
  - The only symbols that may differ between the base build and the patched build are in packages we edit

## Publishing

CI publishes an image when an annotated `X.Y.Z-tpuf.N` tag is pushed.
Merging a PR does not publish anything.

After the PR is merged into `tpuf-<version>-base`:

```sh
git fetch origin
git tag -a 7.82.2-tpuf.2 -m 7.82.2-tpuf.2 origin/tpuf-7.82.2-base
git push origin 7.82.2-tpuf.2
```

Then approve the `publish-derived-images` run under Actions. It builds against
the mirrored vendor image in GAR, pushes to
`us-central1-docker.pkg.dev/turbopuffer-onprem/derived/datadog-agent-tpuf`,
scans, copies to ECR and ACR, then signs and attests all three. The digest and the three image
references are in the job summary. Pin them in the chart.

Some additional info:
- The tag must be annotated. The workflow rejects lightweight tags.
- Push the tag by name. `git push --tags` would also push the upstream release tags.
- Tags in GAR and ECR are immutable, so a reused tag fails at the first push. To rebuild the same release, bump `N`.

## Verifying a published image

Cosign v3 and the digest from the release note. Each image in GAR, ECR and ACR
carries the same digest, signature and attestations.

```sh
ref=us-central1-docker.pkg.dev/turbopuffer-onprem/derived/datadog-agent-tpuf@sha256:<digest>
cosign verify --key cosign-derived-builds.pub --insecure-ignore-tlog=true "$ref"
cosign verify-attestation --key cosign-derived-builds.pub --insecure-ignore-tlog=true --type slsaprovenance1 "$ref"
cosign verify-attestation --key cosign-derived-builds.pub --insecure-ignore-tlog=true --type spdxjson "$ref"
```

- The tlog flag is required because the signatures name no transparency log.
- The provenenace names the fork commit, the tag, and vendor base digest. To confirm the image's relation to the Datadog release, 
compare `crane manifest "$ref"` with `crane manifest gcr.io/datadoghq/agent@<vendor digest>`.

## Moving the base

1. Cut `tpuf-<version>` from the upstream tag and cherry-pick the fork commits.
2. Update `VENDOR_DIGEST` in `Dockerfile` and `build.sh` from `crane digest gcr.io/datadoghq/agent:<version>`.
3. Update the Go version and both tarball checksums in `Dockerfile` from the tag's `.go-version` and `https://go.dev/dl/?mode=json`.
