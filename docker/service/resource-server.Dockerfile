#nginx.Dockerfile

WORKDIR /app

ADD resource-server-nginx.conf /etc/nginx/conf.d/default.conf
COPY docker/dist/reaper .

EXPOSE 80
