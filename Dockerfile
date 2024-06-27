FROM golang:1.22-alpine AS root-deps

WORKDIR /app
COPY go.mod go.sum ./
COPY controller/go.mod controller/go.sum ./controller/
RUN go mod download

FROM golang:1.22-alpine AS builder

WORKDIR /app
COPY --from=root-deps /go/pkg /go/pkg
COPY controller/go.mod controller/go.sum ./controller/
WORKDIR /app/controller
RUN go mod download
COPY controller/ .
RUN go build -o /app/main


FROM alpine:latest
WORKDIR /root/
COPY --from=builder /app/main .
RUN chmod +x ./main
ENTRYPOINT ["./main"]