FROM golang:1.25-alpine AS builder

WORKDIR /app

ENV CGO_ENABLED=0
ENV GOOS=linux
ENV GOARCH=amd64

RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -trimpath -ldflags="-s -w" -o main ./cmd/api

FROM gcr.io/distroless/base-debian12

WORKDIR /

COPY --from=builder /app/main /main

EXPOSE 8080

ENTRYPOINT ["/main"]