# syntax=docker/dockerfile:1
FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/voice-rooms ./cmd/api

FROM alpine:3.22
RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S app && adduser -S -G app app \
    && mkdir /data && chown app:app /data
COPY --from=build /out/voice-rooms /usr/local/bin/voice-rooms
USER app
EXPOSE 8080
VOLUME ["/data"]
ENTRYPOINT ["voice-rooms"]

