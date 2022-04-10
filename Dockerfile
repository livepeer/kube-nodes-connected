FROM golang:1.18 AS builder

WORKDIR /app
COPY go.mod go.mod
COPY go.sum go.sum
RUN go mod download
COPY kube-nodes-connected.go kube-nodes-connected.go
RUN go build kube-nodes-connected.go

FROM debian:11-slim
COPY --from=builder /app/kube-nodes-connected /usr/bin/kube-nodes-connected
CMD kube-nodes-connected
