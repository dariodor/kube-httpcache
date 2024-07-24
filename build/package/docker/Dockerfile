FROM        golang:1.20-bullseye AS builder

WORKDIR     /workspace
COPY        . .
RUN         CGO_ENABLED=0 GOOS=linux \
            go build \
                -installsuffix cgo \
                -o kube-httpcache \
                -a cmd/kube-httpcache/main.go

FROM        golang:1.20-bullseye AS builder-exporter
ENV         EXPORTER_VERSION=1.6.1

WORKDIR     /workspace
RUN         apt-get update \
            && apt-get install -y \
               git
RUN         git clone https://github.com/jonnenauha/prometheus_varnish_exporter.git --depth 1 --branch ${EXPORTER_VERSION}
WORKDIR     /workspace/prometheus_varnish_exporter
RUN         go build


FROM        debian:bullseye-slim AS final

LABEL       MAINTAINER="Martin Helmich <m.helmich@mittwald.de>"

WORKDIR     /

ARG PKG_COMMIT=10da6a585eb7d8defe9d273a51df5b133500eb6b
ARG VARNISH_VERSION=6.0.13
ARG DIST_SHA512=3bdb4f04bdb22789ebe04a1e57dc814a7d7e642456cce2696f7e05fe557a277f18d5dc4a2df22a27fa9445447af3356ebdb3c5d63c01bb32d9bff7881aa8a703

RUN set -e; \
    BASE_PKGS="curl dpkg-dev debhelper devscripts equivs git pkg-config apt-utils fakeroot"; \
    export DEBIAN_FRONTEND=noninteractive; \
    export DEBCONF_NONINTERACTIVE_SEEN=true; \
    tmpdir="$(mktemp -d)"; \
    cd "$tmpdir"; \
    apt-get update; \
    apt-get install -y --no-install-recommends $BASE_PKGS; \
    git clone https://github.com/varnishcache/pkg-varnish-cache.git; \
    cd pkg-varnish-cache; \
    git checkout "$PKG_COMMIT"; \
    rm -rf .git; \
    curl -f "https://varnish-cache.org/downloads/varnish-$VARNISH_VERSION.tgz" -o $tmpdir/orig.tgz; \
    echo "$DIST_SHA512  $tmpdir/orig.tgz" | sha512sum -c -; \
    tar xavf $tmpdir/orig.tgz --strip 1; \
    sed -i -e "s|@VERSION@|$VARNISH_VERSION|"  "debian/changelog"; \
    mk-build-deps --install --tool="apt-get -o Debug::pkgProblemResolver=yes --yes" debian/control; \
    sed -i '' debian/varnish*; \
    dpkg-buildpackage -us -uc -j"$(nproc)"; \
    apt-get -y --no-install-recommends install ../*.deb; \
    apt-get -y purge --auto-remove varnish-build-deps $BASE_PKGS; \
    mkdir /pkgs; \
    mv ../*dev*.deb /pkgs; \
    rm -rf /var/lib/apt/lists/* "$tmpdir"; \
    rm -rf /var/cache/*

RUN         mkdir /exporter && chown varnish /exporter

COPY        --from=builder /workspace/kube-httpcache .
COPY        --from=builder-exporter /workspace/prometheus_varnish_exporter/prometheus_varnish_exporter /exporter/

ENTRYPOINT  [ "/kube-httpcache" ]
