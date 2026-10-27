# new-api / QuantumNous: keep this version in sync with package-lock.json and capture version.
FROM mcr.microsoft.com/playwright:v1.64.1-noble

USER root
ENV NODE_ENV=production PLAYWRIGHT_BROWSERS_PATH=/ms-playwright
# Match Compose and socket ownership; the base image's pwuser may be UID 1001.
RUN apt-get update \
    && apt-get install -y --no-install-recommends fonts-noto-core fonts-noto-cjk fonts-noto-mono tini util-linux \
    && rm -rf /var/lib/apt/lists/* \
    && mkdir -p /app /run/quality-renderer \
    && chown 1000:1000 /app /run/quality-renderer
WORKDIR /app
COPY --chown=1000:1000 package.json package-lock.json ./
USER 1000:1000
RUN npm ci --omit=dev --ignore-scripts --no-audit --no-fund
COPY --chown=1000:1000 src ./src
RUN sed -i 's/\r$//' /app/src/kubernetes-worker.sh \
    && chmod 0555 /app/src/kubernetes-worker.sh
COPY --chown=1000:1000 tests ./tests
COPY --chown=1000:1000 integration ./integration
COPY --chown=1000:1000 kubernetes/*.mjs ./kubernetes/
COPY --chown=1000:1000 seccomp_profile.json ./seccomp_profile.json
COPY --chown=1000:1000 THIRD_PARTY_NOTICES.md LICENSE.apache-2.0 ./
EXPOSE 3000
CMD ["node", "src/gateway.mjs"]
