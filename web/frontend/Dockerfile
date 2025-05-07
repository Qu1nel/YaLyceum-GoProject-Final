FROM nginx:1.28.0-alpine-slim

RUN rm /etc/nginx/conf.d/default.conf

WORKDIR /usr/share/nginx/html

COPY nginx.conf /etc/nginx/conf.d/default.conf

COPY ./index.html .
COPY ./style.css .
COPY ./app.js .

# COPY ./assets /usr/share/nginx/html/assets

EXPOSE 80

CMD ["nginx", "-g", "daemon off;"]