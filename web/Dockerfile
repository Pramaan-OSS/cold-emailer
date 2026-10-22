# syntax=docker/dockerfile:1
FROM node:22-alpine AS build
ENV CI=true
RUN corepack enable && corepack prepare pnpm@10.26.2 --activate
WORKDIR /app/web
COPY web/package.json web/pnpm-lock.yaml web/pnpm-workspace.yaml ./
RUN pnpm install --frozen-lockfile
COPY web/ ./
RUN pnpm build

# Stage 2: serve the built SPA from nginx with a history fallback. The
# entrypoint renders /config.js from container env at startup.
FROM nginx:1.27-alpine
RUN apk add --no-cache curl jq
COPY web/nginx.conf /etc/nginx/conf.d/default.conf
COPY web/nginx-security-headers.conf /etc/nginx/coldemail-security-headers.conf
COPY web/nginx-mail-preview-headers.conf /etc/nginx/coldemail-mail-preview-headers.conf
RUN nginx -t
COPY --from=build /app/web/dist /usr/share/nginx/html
# public/ files keep their checkout mode through the build; on a filesystem
# without POSIX permissions (exFAT/NTFS mounts) that is 0700 and nginx 403s.
RUN chmod -R a+rX /usr/share/nginx/html
COPY web/docker-entrypoint.sh /docker-entrypoint.d/40-coldemail-config.sh
RUN chmod +x /docker-entrypoint.d/40-coldemail-config.sh
EXPOSE 80
