# syntax=docker/dockerfile:1.7
#
# Overlay build for the turbopuffer Datadog agent fork.
#
# Stage "build" compiles the core agent binary twice inside the vendor image
# at the exact release tag: once from the unpatched base sources and once from
# the fork sources. tpuf/compile.sh then runs the differential gate between the
# vendor binary, the base build and the patched build, and strips the patched
# binary the way the vendor does.
#
# The final stage is the vendor image by digest plus one layer that holds the
# patched binary and a notice. Every lower layer digest equals the vendor image.
#
# Build with tpuf/build.sh. It exports the two source trees with git archive
# into the build context as ./patched and ./base.

ARG BASE_IMAGE=gcr.io/datadoghq/agent
ARG BASE_DIGEST=sha256:2104f06e8a2865a7f558a8a6c887c4c0cb84bb587730fd744b62a92099dbdf91

FROM ${BASE_IMAGE}@${BASE_DIGEST} AS build

ARG DEBIAN_FRONTEND=noninteractive
RUN apt-get update \
 && apt-get install -y --no-install-recommends gcc libc6-dev libsystemd-dev binutils ca-certificates curl \
 && rm -rf /var/lib/apt/lists/*

# Go release named by the tag's .go-version. Checksums come from
# https://go.dev/dl/?mode=json and are pinned here.
ARG TARGETARCH
RUN set -eu; \
    case "${TARGETARCH}" in \
      amd64) sha=708effb774be8237570d0add163225abbdfaf4fca28b2611df167beba4feef89 ;; \
      arm64) sha=d0507e9e9d7fe012aae570108cbd76c15de879e17130ab8cb90d4d7445cb1f2e ;; \
      *) echo "unsupported TARGETARCH ${TARGETARCH}" >&2; exit 1 ;; \
    esac; \
    curl -fsSL -o /tmp/go.tar.gz "https://go.dev/dl/go1.26.6.linux-${TARGETARCH}.tar.gz"; \
    echo "${sha}  /tmp/go.tar.gz" | sha256sum -c -; \
    tar -C /usr/local -xzf /tmp/go.tar.gz; \
    rm /tmp/go.tar.gz

ENV PATH=/usr/local/go/bin:/root/go/bin:$PATH \
    GOTOOLCHAIN=local \
    GOFLAGS=-mod=readonly \
    GOPATH=/root/go \
    CGO_ENABLED=1

ARG TPUF_VERSION
ARG TPUF_COMMIT
ARG BASE_VERSION
ARG BASE_COMMIT

WORKDIR /src
COPY base /src/base
COPY patched /src/patched

RUN --mount=type=cache,id=tpuf-gomod,target=/root/go/pkg/mod \
    --mount=type=cache,id=tpuf-gobuild,target=/root/.cache/go-build \
    TPUF_VERSION="${TPUF_VERSION}" TPUF_COMMIT="${TPUF_COMMIT}" \
    BASE_VERSION="${BASE_VERSION}" BASE_COMMIT="${BASE_COMMIT}" \
    /src/patched/tpuf/compile.sh

FROM ${BASE_IMAGE}@${BASE_DIGEST}

ARG BASE_DIGEST
ARG TPUF_VERSION
ARG TPUF_COMMIT

LABEL org.opencontainers.image.source="https://github.com/turbopuffer/datadog-agent" \
      org.opencontainers.image.version="${TPUF_VERSION}" \
      org.opencontainers.image.revision="${TPUF_COMMIT}" \
      org.opencontainers.image.base.name="gcr.io/datadoghq/agent" \
      org.opencontainers.image.base.digest="${BASE_DIGEST}"

# One layer: the patched agent binary and the modification notice.
COPY --from=build --chown=0:0 /out/rootfs/ /
