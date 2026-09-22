FROM node:24-alpine AS build
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci --no-audit --no-fund
COPY . .
ARG VITE_API_URL=/api/v1
ENV VITE_API_URL=$VITE_API_URL
ENV NODE_ENV=production
RUN npm run build && npm cache clean --force

FROM alpine:3.21 AS production
ENV API_BASE_URL=/api/v1
RUN apk add --no-cache nginx gettext \
  && mkdir -p /usr/share/nginx/html /tmp/nginx/client /tmp/nginx/proxy \
  && chown -R nginx:nginx /usr/share/nginx/html /tmp/nginx
COPY nginx-main.conf /etc/nginx/nginx.conf
COPY nginx.conf /etc/nginx/default.conf.template
COPY docker-entrypoint.sh /docker-entrypoint.sh
COPY --chown=nginx:nginx --from=build /app/dist /usr/share/nginx/html
USER nginx
EXPOSE 8080
HEALTHCHECK --interval=10s --timeout=3s --retries=5 --start-period=5s \
  CMD wget -qO- http://127.0.0.1:8080/health >/dev/null || exit 1
ENTRYPOINT ["/bin/sh", "/docker-entrypoint.sh"]
CMD ["nginx", "-g", "daemon off;"]
