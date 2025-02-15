# syntax=docker/dockerfile:1

FROM golang:1.24 AS build

WORKDIR /build

COPY *.* ./
COPY /files/*.* ./files/
COPY /posts/*.* ./posts/
COPY /templates/*.* ./templates/

RUN CGO_ENABLED=0 GOOS=linux go build -o golb


FROM alpine:3.21 AS app

WORKDIR /app

COPY /files/*.* ./files/
COPY /posts/*.* ./posts/
COPY /templates/*.* ./templates/
COPY /LICENSE ./LICENSE
COPY /README.md ./README.md
COPY --from=build /build/golb ./golb

EXPOSE 8080

ENTRYPOINT ["/golb"]