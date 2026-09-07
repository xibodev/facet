FROM golang:1.25.7-bookworm AS go
FROM node:24.14.0-bookworm
COPY --from=go /usr/local/go /usr/local/go
ENV PATH="/home/facet/.facet/bin:/usr/local/go/bin:${PATH}" \
    HOME=/home/facet \
    REMOTION_BROWSER_EXECUTABLE=/usr/bin/chromium \
    PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH=/usr/bin/chromium
RUN apt-get update && apt-get install -y --no-install-recommends \
    ffmpeg chromium fonts-dejavu-core ca-certificates git bash \
    && rm -rf /var/lib/apt/lists/* \
    && npm install --global opencode-ai@1.18.29 \
    && useradd --create-home --uid 10001 facet
RUN chown -R facet:facet /home/facet
COPY .release-harness/docker/tools/package.json /opt/uat/package.json
RUN npm install --prefix /opt/uat --omit=dev --ignore-scripts --no-audit --no-fund --cache /tmp/uat-npm-cache
COPY scripts/uat*.mjs scripts/probe-artifact* /opt/uat/
COPY --chown=facet:facet . /opt/facet-source
USER facet
WORKDIR /opt/facet-source
# Exercise the product installer, not a second test-only installation path.
RUN bash install.sh > /home/facet/uat-install-build.log 2>&1 \
    || (cat /home/facet/uat-install-build.log && exit 1)
USER root
# Test-only forwarding entry point and reference fingerprint remain root-owned.
RUN mkdir -p /opt/uat/bin \
    && chmod 755 /opt/uat/uat-facet-shim.mjs \
    && ln -s ../uat-facet-shim.mjs /opt/uat/bin/facet \
    && sha256sum /home/facet/.facet/bin/facet > /opt/uat/facet.sha256
ENV PATH="/opt/uat/bin:${PATH}"
USER facet
WORKDIR /home/facet
RUN mkdir -p /home/facet/studio /home/facet/uat-evidence
ENTRYPOINT ["node", "/opt/uat/uat-entrypoint.mjs"]
