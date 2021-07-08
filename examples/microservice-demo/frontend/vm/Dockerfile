FROM nginx:1.16.0

WORKDIR /frontend

ADD dist/ /frontend/

ADD nginx.conf /etc/nginx/conf.d/default.conf

EXPOSE 80