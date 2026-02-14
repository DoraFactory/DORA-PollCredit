FROM nginx:1.27-alpine

COPY payment-app/nginx.conf /etc/nginx/conf.d/default.conf
COPY payment-app /usr/share/nginx/html

EXPOSE 80
