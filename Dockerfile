FROM --platform=linux/amd64 golang:1.22-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY src .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o main

FROM --platform=linux/amd64 alpine:latest

WORKDIR /app

RUN apk add --no-cache ca-certificates postgresql-client

COPY --from=builder /app/main .
RUN chmod +x /app/main
COPY base.html .
COPY puneet.pub .
COPY schema.sql .

EXPOSE 8080

CMD ["/app/main"] 