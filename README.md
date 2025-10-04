# SkyglowNotificationServer
 A server to operate Skyglow Notification client


## What is this for?
This is a server that operated SGN clients, to send notifications and such. It's also decentralized, woo.

## Why?
APNS costs 99$/year, and it's unknown on how long it will last on legacy iOS. Aswell, we can't impersonate apps that have died, to give proper notifications. (and also for apps like snapchat that need notifications for realtime)

## Isn't this a little too much effort for saving 99$/year
yes.

## How can I run this server?
1. Create keys
```openssl req -x509 -newkey rsa:4096 -keyout /opt/sgn/keys/server_private_key.pem -out /opt/sgn/keys/server_public_key.pem -days 7300 -nodes```
2. Create Docker Compose (replace your_server_address & password with your info)
```
services:
  postgresql:
    image: docker.io/library/postgres:17-alpine
    restart: unless-stopped
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -d $${POSTGRES_DB} -U $${POSTGRES_USER}"]
      start_period: 20s
      interval: 30s
      retries: 5
      timeout: 5s
    volumes:
      - /opt/sgn/db:/var/lib/postgresql/data
    environment:
      POSTGRES_PASSWORD: password
      POSTGRES_USER: skyglownotify
      POSTGRES_DB: skyglownotify
  server:
    image: ghcr.io/preloading/skyglownotificationserver:dev
    environment:
      - SGN_SERVER_ADDRESS={{YOUR_SERVER_ADDRESS}}
      - SGN_TCP_PORT=21138
      - SGN_KEY_PATH=/config/keys/
      - SGN_DB_TYPE=postgres
      - "SGN_DB_DSN=postgresql://skyglownotify:password@postgresql/skyglownotify"

    volumes:
      - /opt/sgn/keys:/config/keys/
    depends_on:
      postgresql:
        condition: service_healthy
    ports:
      - "21138:21138"
      - "3023:7878"
```
3. Configure the server txt record
With your SERVER_ADDRESS you set, create a TXT record the looks like _sgn.{{SERVER_ADDRESS}}, with the following data:
```
"tcp_addr=tcp.sgn.example.com tcp_port=7373 http_addr=https://sgn.example.com"
```
each pointing to your server.