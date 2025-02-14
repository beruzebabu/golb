# syntax=docker/dockerfile:1

FROM golang:1.24

WORKDIR /app

COPY *.* ./
COPY /files/*.* ./files/
COPY /posts/*.* ./posts/
COPY /templates/*.* ./templates/

RUN CGO_ENABLED=0 GOOS=linux go build -o /golb

CMD ["/golb"]