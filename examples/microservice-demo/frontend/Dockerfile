FROM ccr.ccs.tencentyun.com/trial-dist/frontend-base:0.1  As builder

WORKDIR /frontend

COPY . /frontend

RUN make install-frontend-dep build-frontend


FROM nginx:1.16.0

WORKDIR /frontend

COPY --from=builder /frontend/dist /frontend/

COPY --from=builder /frontend/nginx.conf /etc/nginx/conf.d/default.conf

EXPOSE 80