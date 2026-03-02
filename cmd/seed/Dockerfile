FROM golang:1.25-alpine AS builder

WORKDIR /app

ENV CGO_ENABLED=0
ENV GOOS=linux
ENV GOARCH=amd64

RUN apk add --no-cache git

COPY go.mod go.sum* ./
RUN go mod tidy

COPY . .

RUN go build -trimpath -ldflags="-s -w" -o seed ./cmd/seed

FROM gcr.io/distroless/base-debian12

WORKDIR /

COPY --from=builder /app/seed /seed

ENTRYPOINT ["/seed"]