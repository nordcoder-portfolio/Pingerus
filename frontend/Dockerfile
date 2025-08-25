# build stage
FROM node:20-alpine AS build
WORKDIR /app
COPY frontend/package*.json ./
RUN npm ci
COPY frontend/ .
RUN npm run build

# runtime stage
FROM nginx:1.27-alpine
COPY --from=build /app/dist /usr/share/nginx/html

# ВАЖНО: пути из корня контекста!
COPY frontend/deploy/nginx.conf /etc/nginx/conf.d/default.conf
COPY frontend/deploy/entrypoint.sh /docker-entrypoint.d/99-app-config.sh
RUN chmod +x /docker-entrypoint.d/99-app-config.sh

EXPOSE 80
