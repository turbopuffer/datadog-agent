#!/usr/bin/env bash
# Build the turbopuffer Datadog agent overlay image.
#
# Usage:
#   tpuf/build.sh [--push] [--image REPO] [--tag TAG]
#
# TAG defaults to the annotated tag on HEAD and must look like 7.82.2-tpuf.1.
# The base release is TAG with the -tpuf.N suffix removed. Its commit must be
# reachable as tag <base>, or the script fetches it from upstream.
#
# Environment:
#   VENDOR_IMAGE   vendor image repository. Default gcr.io/datadoghq/agent.
#                Set to turbopuffer.azurecr.io/mirror/datadoghq/agent in CI.
#   VENDOR_DIGEST  vendor image index digest for <base>. Default pinned below.
#   COSIGN_PUB   public key that signed VENDOR_IMAGE. Verification runs only when
#                VENDOR_IMAGE is a turbopuffer registry.
#   PLATFORM     default linux/amd64.
set -euo pipefail

here=$(cd "$(dirname "$0")" && pwd)
repo=$(cd "${here}/.." && pwd)

UPSTREAM_URL=https://github.com/DataDog/datadog-agent.git
VENDOR_IMAGE=${VENDOR_IMAGE:-gcr.io/datadoghq/agent}
VENDOR_DIGEST=${VENDOR_DIGEST:-sha256:2104f06e8a2865a7f558a8a6c887c4c0cb84bb587730fd744b62a92099dbdf91}
COSIGN_PUB=${COSIGN_PUB:-${here}/cosign.pub}
PLATFORM=${PLATFORM:-linux/amd64}

push=0
image=${IMAGE:-turbopuffer/datadog-agent-tpuf}
tag=""
while [ $# -gt 0 ]; do
  case "$1" in
    --push) push=1 ;;
    --image) image=$2; shift ;;
    --tag) tag=$2; shift ;;
    *) echo "unknown argument: $1" >&2; exit 2 ;;
  esac
  shift
done

log() { printf '==> %s\n' "$*"; }
fail() { printf 'FAIL: %s\n' "$*" >&2; exit 1; }

cd "${repo}"

if [ -z "${tag}" ]; then
  tag=$(git describe --tags --exact-match HEAD 2>/dev/null) || fail "HEAD carries no tag, pass --tag"
fi
[[ "${tag}" =~ ^[0-9]+\.[0-9]+\.[0-9]+-tpuf\.[0-9]+$ ]] || fail "tag ${tag} does not match X.Y.Z-tpuf.N"
base_version=${tag%%-tpuf.*}

tpuf_commit=$(git rev-parse HEAD)
if ! base_commit=$(git rev-parse --verify --quiet "refs/tags/${base_version}^{commit}"); then
  log "fetching upstream tag ${base_version}"
  git fetch --no-tags --depth=1 "${UPSTREAM_URL}" "refs/tags/${base_version}:refs/tags/${base_version}"
  base_commit=$(git rev-parse --verify "refs/tags/${base_version}^{commit}")
fi
log "tag ${tag} at ${tpuf_commit}, base ${base_version} at ${base_commit}"

case "${VENDOR_IMAGE}" in
  *.azurecr.io/*|*.dkr.ecr.*.amazonaws.com/*|*-docker.pkg.dev/*)
    log "verifying ${VENDOR_IMAGE}@${VENDOR_DIGEST} against ${COSIGN_PUB}"
    cosign verify --key "${COSIGN_PUB}" --insecure-ignore-tlog=true "${VENDOR_IMAGE}@${VENDOR_DIGEST}" > /dev/null
    ;;
  *)
    log "base ${VENDOR_IMAGE} is not a turbopuffer registry, skipping signature verification"
    ;;
esac

context=$(mktemp -d)
trap 'rm -rf "${context}"' EXIT
mkdir -p "${context}/base" "${context}/patched"
git archive --format=tar "${base_commit}" | tar -x -C "${context}/base"
git archive --format=tar "${tpuf_commit}" | tar -x -C "${context}/patched"

ref="${image}:${tag}"
log "building ${ref} for ${PLATFORM}"
build_args=(
  --platform "${PLATFORM}"
  --provenance=false
  --sbom=false
  -f "${here}/Dockerfile"
  --build-arg "VENDOR_IMAGE=${VENDOR_IMAGE}"
  --build-arg "VENDOR_DIGEST=${VENDOR_DIGEST}"
  --build-arg "TPUF_VERSION=${tag}"
  --build-arg "TPUF_COMMIT=${tpuf_commit}"
  --build-arg "BASE_VERSION=${base_version}"
  --build-arg "BASE_COMMIT=${base_commit}"
  -t "${ref}"
)
if [ "${push}" = 1 ]; then
  docker buildx build "${build_args[@]}" --push "${context}"
  digest=$(crane digest "${ref}")
  log "pushed ${ref}@${digest}"
  if [ -n "${GITHUB_STEP_SUMMARY:-}" ]; then
    printf '**%s**\n\n`%s@%s`\n' "${ref}" "${image}" "${digest}" >> "${GITHUB_STEP_SUMMARY}"
  fi
  if [ -n "${GITHUB_OUTPUT:-}" ]; then
    printf 'digest=%s\nref=%s\n' "${digest}" "${ref}" >> "${GITHUB_OUTPUT}"
  fi
  printf '%s\n' "${digest}"
else
  docker buildx build "${build_args[@]}" --load "${context}"
  log "built ${ref}"
  docker run --rm --platform "${PLATFORM}" --entrypoint agent "${ref}" version
fi
