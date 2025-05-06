FROM nginx:1.28.0-alpine-slim

# Удаляем стандартную конфигурацию Nginx
RUN rm /etc/nginx/conf.d/default.conf

# Копируем нашу кастомную конфигурацию Nginx (создадим ее на следующем шаге)
COPY nginx.conf /etc/nginx/conf.d/default.conf

# Копируем статические файлы фронтенда (index.html, style.css, app.js)
# WORKDIR /usr/share/nginx/html # Это рабочая директория по умолчанию для nginx:alpine
COPY ./index.html /usr/share/nginx/html/index.html
COPY ./style.css /usr/share/nginx/html/style.css
COPY ./app.js /usr/share/nginx/html/app.js

# (Опционально) Если будут другие ассеты (картинки, шрифты), скопировать их тоже
# COPY ./assets /usr/share/nginx/html/assets

# Nginx по умолчанию слушает порт 80 внутри контейнера
EXPOSE 80

# Команда для запуска Nginx (уже есть в базовом образе)
# CMD ["nginx", "-g", "daemon off;"]