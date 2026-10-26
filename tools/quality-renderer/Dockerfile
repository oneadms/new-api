# new-api / QuantumNous: keep this version in sync with package-lock.json and capture version.
FROM mcr.microsoft.com/playwright:v1.64.1-noble

USER root
ENV NODE_ENV=production PLAYWRIGHT_BROWSERS_PATH=/ms-playwright
RUN apt-get update \
    && apt-get install -y --no-install-recommends fonts-noto-core fonts-noto-cjk fonts-noto-mono \
    && rm -rf /var/lib/apt/lists/* \
    && mkdir -p /app /run/quality-renderer \
    && chown pwuser:pwuser /app /run/quality-renderer
WORKDIR /app
COPY --chown=pwuser:pwuser package.json package-lock.json ./
USER pwuser
RUN npm ci --omit=dev --ignore-scripts --no-audit --no-fund
COPY --chown=pwuser:pwuser src ./src
COPY --chown=pwuser:pwuser tests ./tests
COPY --chown=pwuser:pwuser integration ./integration
COPY --chown=pwuser:pwuser THIRD_PARTY_NOTICES.md LICENSE.apache-2.0 ./
EXPOSE 3000
CMD ["node", "src/gateway.mjs"]
