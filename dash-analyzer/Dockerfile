FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
COPY third_party/openhours ./third_party/openhours
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /server ./cmd/server
FROM alpine:3.23
RUN apk add --no-cache ca-certificates tzdata chromium && adduser -D -u 10001 app && mkdir /data && chown app /data
WORKDIR /app
COPY --from=build /server /app/server
COPY web /app/web
USER app
ENV DATA_DIR=/data LISTEN_ADDR=0.0.0.0:8080
EXPOSE 8080
ENTRYPOINT ["/app/server"]
