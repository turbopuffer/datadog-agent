#!/usr/bin/env bash
# Sign one image by digest, attach its attestations, verify all of it.
#
# Usage:
#   tpuf/sign.sh --key KEY --pub PUB [--sbom FILE] [--provenance FILE] [--http] REF@DIGEST
#
# KEY is a cosign key reference: gcpkms://... in the publish workflow, a key
# file in tpuf-ci. PUB is the matching public key. The committed signing config
# names no transparency log or timestamp authority and the trusted root is
# empty, so cosign contacts only the registry and, for a KMS key, Cloud KMS.
# Verification passes --insecure-ignore-tlog for the same reason. --http allows
# a plain-HTTP registry, the job-local registry in tpuf-ci.
set -euo pipefail

here=$(cd "$(dirname "$0")" && pwd)
key="" pub="" sbom="" provenance="" ref=""
http=()
while [ $# -gt 0 ]; do
  case "$1" in
    --key) key=$2; shift ;;
    --pub) pub=$2; shift ;;
    --sbom) sbom=$2; shift ;;
    --provenance) provenance=$2; shift ;;
    --http) http=(--allow-http-registry) ;;
    -*) echo "unknown argument: $1" >&2; exit 2 ;;
    *) ref=$1 ;;
  esac
  shift
done
if [ -z "${key}" ] || [ -z "${pub}" ] || [ -z "${ref}" ]; then
  echo "usage: $0 --key KEY --pub PUB [--sbom FILE] [--provenance FILE] [--http] REF@DIGEST" >&2
  exit 2
fi
[[ "${ref}" == *@sha256:* ]] || { echo "sign by digest, not by tag: ${ref}" >&2; exit 2; }

sign_opts=(--yes
  --signing-config "${here}/cosign-signing-config.json"
  --trusted-root "${here}/cosign-trusted-root.json"
  --key "${key}" ${http[@]+"${http[@]}"})
verify_opts=(--key "${pub}" --insecure-ignore-tlog=true ${http[@]+"${http[@]}"})

echo "==> signing ${ref}"
cosign sign "${sign_opts[@]}" "${ref}"
cosign verify "${verify_opts[@]}" "${ref}" > /dev/null

attest() {
  local type=$1 predicate=$2
  echo "==> attesting ${type} on ${ref}"
  cosign attest "${sign_opts[@]}" --type "${type}" --predicate "${predicate}" "${ref}"
  cosign verify-attestation "${verify_opts[@]}" --type "${type}" "${ref}" > /dev/null
}
[ -z "${sbom}" ] || attest spdxjson "${sbom}"
[ -z "${provenance}" ] || attest slsaprovenance1 "${provenance}"
