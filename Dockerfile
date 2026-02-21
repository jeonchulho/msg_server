FROM golang:1.25-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG CMD_DIR=./cmd/chat
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/msg_server "${CMD_DIR}"

FROM alpine:3.20

RUN adduser -D -u 10001 appuser

COPY --from=builder /out/msg_server /usr/local/bin/msg_server

USER appuser

ENTRYPOINT ["/usr/local/bin/msg_server"]
