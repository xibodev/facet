FROM golang:1.25.7-bookworm AS go
FROM node:24.14.0-bookworm
ARG NPM_REGISTRY=https://registry.npmjs.org
COPY --from=go /usr/local/go /usr/local/go
ENV npm_config_registry=${NPM_REGISTRY}
ENV PATH="/home/facet/.facet/bin:/usr/local/go/bin:${PATH}" \
    HOME=/home/facet \
    REMOTION_BROWSER_EXECUTABLE=/usr/bin/chromium \
    PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH=/usr/bin/chromium
RUN apt-get update && apt-get install -y --no-install-recommends \
    ffmpeg chromium fonts-dejavu-core ca-certificates git bash curl unzip zip \
    && rm -rf /var/lib/apt/lists/*
RUN npm install --global opencode-ai@1.18.29 --registry="${NPM_REGISTRY}"
RUN useradd --create-home --uid 10001 facet
RUN chown -R facet:facet /home/facet
COPY .release-harness/docker/tools/package.json /opt/uat/package.json
RUN npm install --prefix /opt/uat --registry="${NPM_REGISTRY}" \
    --omit=dev --ignore-scripts --no-audit --no-fund --cache /tmp/uat-npm-cache
COPY scripts/uat*.mjs scripts/probe-artifact* /opt/uat/
COPY --chown=facet:facet . /opt/facet-source
WORKDIR /opt/facet-source
# Exercise the product installer, not a second test-only installation path.
RUN mkdir -p /tmp/facet-release/bin /tmp/facet-release/bundle \
    && go build -trimpath -ldflags "-X main.version=1.0.4" -o /tmp/facet-release/bin/facet ./cmd/facet \
    && go build -trimpath -ldflags "-X main.version=1.0.4" -o /tmp/facet-release/bin/facet-ui ./cmd/facet-ui \
    && go build -trimpath -ldflags "-X main.version=1.0.4" -o /tmp/facet-release/bin/facet-module ./cmd/facet-module \
    && cp -R skills packs agents schemas remotion-composer /tmp/facet-release/bundle/ \
    && cp LICENSE THIRD_PARTY_NOTICES.md /tmp/facet-release/ \
    && cd /tmp/facet-release \
    && zip -qr /tmp/facet-1.0.4-linux-amd64.zip . \
    && cd /tmp \
    && sha256sum facet-1.0.4-linux-amd64.zip > checksums-linux-amd64.txt
RUN bash install.sh --yes --target opencode --project /home/facet/studio --install-dir /home/facet/.facet/runtime --components remotion \
    --archive /tmp/facet-1.0.4-linux-amd64.zip --checksums /tmp/checksums-linux-amd64.txt \
    > /home/facet/uat-install-build.log 2>&1 \
    || (cat /home/facet/uat-install-build.log /home/facet/.facet/logs/install-*.log && exit 1)
USER root
# Test-only forwarding entry point and reference fingerprint remain root-owned.
RUN mkdir -p /opt/uat/bin /home/facet/.facet/bin \
    && chmod 755 /opt/uat/uat-facet-shim.mjs \
    && ln -s ../runtime/bin/facet /home/facet/.facet/bin/facet \
    && ln -s runtime/bundle /home/facet/.facet/bundle \
    && ln -s ../uat-facet-shim.mjs /opt/uat/bin/facet \
    && sha256sum /home/facet/.facet/bin/facet > /opt/uat/facet.sha256 \
    && chown -R facet:facet /home/facet
ENV PATH="/opt/uat/bin:${PATH}"
USER facet
RUN /home/facet/.facet/bin/facet bundle --target studio --out /home/facet/.facet/bundle
WORKDIR /home/facet
RUN mkdir -p /home/facet/studio /home/facet/uat-evidence
ENTRYPOINT ["node", "/opt/uat/uat-entrypoint.mjs"]
