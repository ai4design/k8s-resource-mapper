FROM golang:1.20-alpine AS builder
WORKDIR /app
COPY . .
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -o k8s-resource-mapper cmd/mapper/main.go

FROM alpine:3.18
WORKDIR /
COPY --from=builder /app/k8s-resource-mapper .
ENTRYPOINT ["/k8s-resource-mapper"]
