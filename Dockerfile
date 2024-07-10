FROM golang:1.22-alpine AS builder

ARG TARGETOS
ARG TARGETARCH

WORKDIR /app/controller
COPY controller/ ./
RUN go mod download
WORKDIR /app
COPY go.mod go.sum main.go ./
RUN go mod download
RUN GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -o /app/main


FROM alpine:latest
WORKDIR /root/
COPY --from=builder /app/main .
RUN chmod +x ./main
ENTRYPOINT [ "./main" ]


