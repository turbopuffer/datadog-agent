#!/usr/bin/env bash
# Runs inside the build stage of tpuf/Dockerfile.
#
# Builds the core agent from /src/base (unpatched release sources) and from
# /src/patched (fork sources) with the vendor binary's Go version, build tags
# and link flags, then gates the patched build against the vendor binary and
# the base build. Writes the shipped files to /out/rootfs.
set -euo pipefail

: "${TPUF_VERSION:?}" "${TPUF_COMMIT:?}" "${BASE_VERSION:?}" "${BASE_COMMIT:?}"

VENDOR=/opt/datadog-agent/bin/agent/agent
INSTALL_PATH=/opt/datadog-agent
EMBEDDED=${INSTALL_PATH}/embedded
MODULE=github.com/DataDog/datadog-agent
OUT=/out
mkdir -p "${OUT}"

# Symbols that may differ between the base build and the patched build.
# Every package the fork edits, and nothing else.
ALLOWED_SYMBOL_DIFF='comp/metadata/host/impl/hosttags|comp/metadata/host/impl/utils|pkg/config/setup|pkg/collector/check/stats|pkg/aggregator'

log() { printf '==> %s\n' "$*"; }
fail() { printf 'FAIL: %s\n' "$*" >&2; exit 1; }

buildinfo() { go version -m "$1"; }

vendor_go=$(buildinfo "${VENDOR}" | head -1 | awk '{print $2}')
local_go=$(go version | awk '{print $3}')
[ "${vendor_go}" = "${local_go}" ] || fail "Go ${local_go} does not match the vendor binary's ${vendor_go}"

TAGS=$(buildinfo "${VENDOR}" | sed -n 's/^[[:space:]]*build[[:space:]]*-tags=//p' | tr ',' ' ')
[ -n "${TAGS}" ] || fail "no build tags found in the vendor binary"
log "build tags: ${TAGS}"

payload_version() {
  awk '$1=="github.com/DataDog/agent-payload/v5"{print $2; exit}' "$1/go.mod" | cut -d+ -f1
}

# Mirrors get_build_flags and get_version_ldflags in tasks/libs/common/utils.py
# for a Linux build with --install-path=/opt/datadog-agent and
# --embedded-path=/opt/datadog-agent/embedded.
build_agent() {
  local src=$1 out=$2 version=$3 commit=$4
  local payload short
  payload=$(payload_version "${src}")
  short=${commit:0:7}
  local ldflags="-X ${MODULE}/pkg/version.Commit=${short}"
  ldflags+=" -X ${MODULE}/pkg/version.FullCommit=${commit}"
  ldflags+=" -X ${MODULE}/pkg/version.AgentVersion=${version}"
  ldflags+=" -X ${MODULE}/pkg/version.AgentPayloadVersion=${payload}"
  ldflags+=" -X ${MODULE}/pkg/version.AgentPackageVersion=${version}"
  ldflags+=" -X ${MODULE}/pkg/version.AgentVersionURLSafe=${version}"
  ldflags+=" -X ${MODULE}/pkg/util/defaultpaths.defaultInstallPath=${INSTALL_PATH}"
  ldflags+=" -X ${MODULE}/pkg/collector/python.pythonHome3=${EMBEDDED}"
  ldflags+=" -r ${EMBEDDED}/lib"
  ldflags+=" -extldflags=-Wl,--version-script=${src}/datadog-agent.map"

  log "building ${version} from ${src}"
  (
    cd "${src}"
    CGO_CFLAGS="-I${EMBEDDED}/include" \
    CGO_LDFLAGS="-L${EMBEDDED}/lib" \
    go build -trimpath -buildvcs=false -tags "${TAGS}" -ldflags "${ldflags}" -o "${out}" ./cmd/agent
  )
}

build_agent /src/base "${OUT}/agent-base" "${BASE_VERSION}" "${BASE_COMMIT}"
build_agent /src/patched "${OUT}/agent-patched" "${TPUF_VERSION}" "${TPUF_COMMIT}"

# Gate 1: module list and build tags equal the vendor binary.
deps_and_tags() { buildinfo "$1" | grep -E '^[[:space:]]*(dep[[:space:]]|build[[:space:]]+-tags=)'; }
log "gate: module list and build tags"
diff <(deps_and_tags "${VENDOR}") <(deps_and_tags "${OUT}/agent-base") || fail "base build differs from the vendor binary in modules or tags"
diff <(deps_and_tags "${VENDOR}") <(deps_and_tags "${OUT}/agent-patched") || fail "patched build differs from the vendor binary in modules or tags"

# Gate 2: dynamic section equal to the vendor binary. Symbol version
# requirements may differ because the vendor links an older glibc, and that is
# accepted: the binary runs inside this same image.
dynamic() { readelf -d "$1" | grep -E 'NEEDED|RUNPATH|RPATH' | sed 's/^[[:space:]]*0x[0-9a-f]*//'; }
log "gate: NEEDED and RUNPATH"
diff <(dynamic "${VENDOR}") <(dynamic "${OUT}/agent-patched") || fail "dynamic section differs from the vendor binary"

# Gate 3: the only symbols that differ between the base and patched builds
# live in the packages the fork edits.
symbols() { go tool nm -sort name "$1" | awk '{print $NF}' | sort -u; }
log "gate: symbol diff between base and patched builds"
symdiff=$(diff <(symbols "${OUT}/agent-base") <(symbols "${OUT}/agent-patched") | grep -E '^[<>]' || true)
printf '%s\n' "${symdiff}" | sed '/^$/d' > "${OUT}/symbol-diff.txt"
unexpected=$(printf '%s\n' "${symdiff}" | sed '/^$/d' | grep -Ev "${ALLOWED_SYMBOL_DIFF}" || true)
if [ -n "${unexpected}" ]; then
  printf '%s\n' "${unexpected}" >&2
  fail "symbols outside the allowed packages differ"
fi
log "symbol diff: $(wc -l < "${OUT}/symbol-diff.txt") symbols, all in allowed packages"

# Gate 4: the stamped version resolves to the intake domain of the base release.
log "gate: version stamp"
version_line=$("${OUT}/agent-patched" version)
printf '%s\n' "${version_line}"
printf '%s\n' "${version_line}" | grep -Eq "^Agent ${TPUF_VERSION//./\\.} " || fail "agent version does not report ${TPUF_VERSION}"
[ "${TPUF_VERSION%%-tpuf.*}" = "${BASE_VERSION}" ] || fail "${TPUF_VERSION} is not a pre-release of ${BASE_VERSION}"

# Ship a stripped binary like the vendor. strip keeps .go.buildinfo, so
# `go version -m` and `agent version` still work on the shipped file.
cp "${OUT}/agent-patched" "${OUT}/agent-patched.debug"
strip "${OUT}/agent-patched"

mkdir -p "${OUT}/rootfs${INSTALL_PATH}/bin/agent"
install -m 0755 "${OUT}/agent-patched" "${OUT}/rootfs${INSTALL_PATH}/bin/agent/agent"
cat > "${OUT}/rootfs${INSTALL_PATH}/NOTICE-turbopuffer" <<EOF
This image is the Datadog Agent ${BASE_VERSION} container image with one file
replaced: ${INSTALL_PATH}/bin/agent/agent.

The replacement binary is built from https://github.com/turbopuffer/datadog-agent
at commit ${TPUF_COMMIT} (version ${TPUF_VERSION}), a fork of
https://github.com/DataDog/datadog-agent at tag ${BASE_VERSION} (commit ${BASE_COMMIT}).
Modified files, per Apache License 2.0 section 4(b), are listed by
  git diff --stat ${BASE_COMMIT}..${TPUF_COMMIT}
in that repository. All other files in this image are unmodified and keep
their original licenses under ${INSTALL_PATH}/LICENSE and ${INSTALL_PATH}/LICENSES.
EOF
log "done"
