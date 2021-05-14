FROM nginx:1.16.0

WORKDIR /zadig-portal

ADD dist/ /zadig-portal/

ADD zadig-nginx.conf /etc/nginx/conf.d/default.conf

EXPOSE 80
