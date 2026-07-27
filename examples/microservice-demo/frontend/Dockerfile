FROM node:22-alpine AS builder

WORKDIR /frontend

COPY package.json yarn.lock ./

RUN corepack enable \
 && yarn install --frozen-lockfile

COPY . /frontend

RUN yarn run build


FROM koderover.tencentcloudcr.com/koderover-demo/nginx:1.16.0

WORKDIR /frontend

COPY --from=builder /frontend/dist /frontend/

COPY --from=builder /frontend/nginx.conf /etc/nginx/conf.d/default.conf

EXPOSE 80
