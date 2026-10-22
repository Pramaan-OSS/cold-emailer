# syntax=docker/dockerfile:1
FROM node:22-alpine AS build
ENV CI=true
RUN corepack enable && corepack prepare pnpm@10.26.2 --activate
WORKDIR /app/admin
COPY admin/package.json admin/pnpm-lock.yaml admin/pnpm-workspace.yaml ./
RUN pnpm install --frozen-lockfile
COPY admin/ ./
RUN pnpm build

# Stage 2: serve the built SPA from nginx with a history fallback. The
# entrypoint renders /config.js from container env at startup.
FROM nginx:1.27-alpine
COPY admin/nginx.conf /etc/nginx/conf.d/default.conf
COPY admin/nginx-security-headers.conf /etc/nginx/coldemail-security-headers.conf
RUN nginx -t
COPY --from=build /app/admin/dist /usr/share/nginx/html
# public/ files keep their checkout mode through the build; on a filesystem
# without POSIX permissions (exFAT/NTFS mounts) that is 0700 and nginx 403s.
RUN chmod -R a+rX /usr/share/nginx/html
COPY admin/docker-entrypoint.sh /docker-entrypoint.d/40-coldemail-config.sh
RUN chmod +x /docker-entrypoint.d/40-coldemail-config.sh
EXPOSE 80
