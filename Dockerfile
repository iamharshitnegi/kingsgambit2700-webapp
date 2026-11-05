# syntax=docker/dockerfile:1.7
FROM node:24-bookworm-slim AS build
WORKDIR /app
COPY package.json package-lock.json ./
RUN --mount=type=cache,target=/root/.npm npm ci
COPY . .

# Public URLs are compiled into the client bundle. Never pass secrets as ARGs.
ARG WEB_DOMAIN=kingsgambit2700.in
ARG API_DOMAIN=api.kingsgambit2700.in
ENV KG_NEXT_STANDALONE=true \
    NEXT_TELEMETRY_DISABLED=1 \
    NEXT_PUBLIC_DEMO_MODE=false \
    NEXT_PUBLIC_SITE_URL=https://${WEB_DOMAIN} \
    NEXT_PUBLIC_API_BASE_URL=https://${API_DOMAIN} \
    NEXT_PUBLIC_REALTIME_URL=wss://${API_DOMAIN}/ws/v1 \
    API_BASE_URL=https://${API_DOMAIN} \
    EDITORIAL_CONTENT_ENABLED=false
RUN npm run build

FROM node:24-bookworm-slim AS frontend
WORKDIR /app
ENV NODE_ENV=production \
    NEXT_TELEMETRY_DISABLED=1 \
    PORT=3000 \
    HOSTNAME=0.0.0.0
COPY --from=build --chown=node:node /app/.next/standalone ./
COPY --from=build --chown=node:node /app/.next/static ./.next/static
COPY --from=build --chown=node:node /app/public ./public
USER node
EXPOSE 3000
HEALTHCHECK --interval=30s --timeout=5s --start-period=30s --retries=3 \
  CMD node -e "fetch('http://127.0.0.1:3000/sign-in').then(r=>process.exit(r.ok?0:1)).catch(()=>process.exit(1))"
CMD ["node", "server.js"]
