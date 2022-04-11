
.PHONY: all
all: build

.PHONY: build
build:
	docker build --platform=linux/amd64 -t livepeerci/kube-nodes-connected .

.PHONY: push
push:
	docker push livepeerci/kube-nodes-connected

.PHONY: build-local
build-local:
	GOOS=linux GOARCH=amd64 go build kube-nodes-connected.go
	docker build --platform linux/amd64 -f Dockerfile.local -t livepeerci/kube-nodes-connected .
