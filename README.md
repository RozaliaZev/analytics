# Сервис сбора аналитики (аудита)
сервис предназначен для сбора и хранения информации о действиях совершенных
пользователями. с какой то периодичностью другие сервисы отправляют в сервис
аналитики сообщения о действиях пользователя, например: пользователь
авторизовался, пользователь изменил свои настройки, добавил аватарку

## Конфигурирование сервиса

Конфигурирование приложения происходит через файф config.yaml

- **db** информация для подключения к базе данных PostgreSQL

- **server/port** дает возможность указать на каком порту сервис слушает запросы

- **gin/mode** режим работы фреймворка gin

```sh
db:
  HOST: localhost
  PORT: 5432
  USER: user
  PASSWORD: password
  DBNAME: base
  SSLMODE: disable

server:
  port: 8080

gin:
  mode: release
```

## Запуск приложения

после заполнения файла config.yaml можно использовать приложение.

можно сделать билд и запустить его, можно просто воспользоваться командой

```sh
cd cmd
go run main.go
```

## Отправка пробного запроса

С помощью Postman сформирует запрос
```sh
POST 'http://localhost:8080/analitycs' \
--header 'X-Tantum-UserAgent: DeviceID=G1752G75-7C56-4G49-BGFA5ACBGC963471;DeviceType=iOS;OsVersion=15.5;AppVersion=4.3 (725)' \
--header 'X-Tantum-Authorization: 2daba111-1e48-4ba1-8753-2daba1119a09' \
--header 'Content-Type: application/json' \
--data-raw '{
 "module" : "settings",
 "type" : "alert",
 "event" : "click",
 "name" : "подтверждение выхода",
 "data" : {"action" : "cancel"}
}'
```

Получаем мгновенный ответ
```sh
{
    "status": "ok"
}
```

Проверим базу данных и запись в ней:
- **time** - время когда было получено сообщение
- **user_id** - значение X-Tantum-Authorization из заголовка запроса
- **data** - json с данными в которых содержаться как заголовки, так и тело запроса

![alt text](https://sun9-46.userapi.com/impg/QbWZQM8cpZjm-dcTBUTBkXlHG4QqumiD9YHPFA/jwxV6vG_EAI.jpg?size=1495x122&quality=95&sign=da7eb7d8d70e0328bd88b3d3e3681d8c&type=album)

### Как реализована работа
Этот код представляет собой веб-сервер, который обрабатывает HTTP-запросы, отправленные на `/analytics`. 

В частности, этот код:

- Считывает конфигурационный файл и устанавливает параметры для подключения к базе данных.
- Создаёт таблицу `analytics` в базе данных, если она не существует.
- Создаёт буферизированный канал `processedRequests` для обработанных запросов.
- Запускает воркер-пулл для обработки запросов.
- Настраивает роутер Gin для обработки запросов на `/analytics`.
- Отправляет запрос в канал для обработки.
- Для каждого запроса в канале воркер-пулл обрабатывает его, сохраняет данные в базе данных и возвращает результаты обработки.
