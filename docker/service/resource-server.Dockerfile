#nginx.Dockerfile

WORKDIR /app

ADD resource-server-nginx.conf /etc/nginx/conf.d/default.conf
ADD docker/dist/reaper .

EXPOSE 80
