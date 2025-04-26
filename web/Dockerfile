# build and compile
FROM node:23.11.0-slim as build-stage
WORKDIR /app
COPY ./kube-jit/ /app/
# ADD wss.crt /etc/ssl/wss.crt
# ENV NODE_EXTRA_CA_CERTS=/etc/ssl/wss.crt
RUN yarn install
RUN yarn run build

# Stage 1, based on Nginx, to have only the compiled app, ready for production with Nginx
FROM nginx:1.28.0-alpine-slim
COPY --from=build-stage /app/dist/ /usr/share/nginx/html
