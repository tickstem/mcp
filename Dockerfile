FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o tsk-mcp ./cmd/tsk-mcp

FROM alpine:latest
RUN apk --no-cache add ca-certificates
COPY --from=builder /app/tsk-mcp /usr/local/bin/tsk-mcp
ENTRYPOINT ["tsk-mcp"]
