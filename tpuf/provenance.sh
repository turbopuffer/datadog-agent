#!/usr/bin/env bash
# Print a SLSA v1 provenance predicate for one overlay build.
#
# Usage:
#   tpuf/provenance.sh --tag TAG --commit SHA --base IMAGE@DIGEST --run-url URL [--platform linux/amd64]
#
# `cosign attest --type slsaprovenance1` wraps the output in an in-toto
# statement whose subject is the image digest.
set -euo pipefail

tag="" commit="" base="" run_url="" platform=linux/amd64
while [ $# -gt 0 ]; do
  case "$1" in
    --tag) tag=$2; shift ;;
    --commit) commit=$2; shift ;;
    --base) base=$2; shift ;;
    --run-url) run_url=$2; shift ;;
    --platform) platform=$2; shift ;;
    *) echo "unknown argument: $1" >&2; exit 2 ;;
  esac
  shift
done
for name in tag commit base run_url; do
  [ -n "${!name}" ] || { echo "--${name//_/-} is required" >&2; exit 2; }
done
[[ "${base}" == *@sha256:* ]] || { echo "--base must carry a sha256 digest: ${base}" >&2; exit 2; }

repo_url=https://github.com/turbopuffer/datadog-agent
workflow=.github/workflows/publish-telemetry-images.yml

jq -n \
  --arg tag "${tag}" \
  --arg commit "${commit}" \
  --arg platform "${platform}" \
  --arg base_uri "${base%%@*}" \
  --arg base_sha "${base##*@sha256:}" \
  --arg repo "${repo_url}" \
  --arg workflow "${workflow}" \
  --arg run_url "${run_url}" \
  --arg started "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  '{
    buildDefinition: {
      buildType: ($repo + "/tpuf/build.sh@v1"),
      externalParameters: {
        tag: $tag,
        workflow: {repository: $repo, ref: ("refs/tags/" + $tag), path: $workflow}
      },
      internalParameters: {platform: $platform},
      resolvedDependencies: [
        {uri: ("git+" + $repo + "@refs/tags/" + $tag), digest: {gitCommit: $commit}},
        {uri: ("oci://" + $base_uri), digest: {sha256: $base_sha}, name: "vendor base image"}
      ]
    },
    runDetails: {
      builder: {id: ($repo + "/" + $workflow + "@refs/tags/" + $tag)},
      metadata: {invocationId: $run_url, startedOn: $started}
    }
  }'
