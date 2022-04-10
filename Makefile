
.PHONY: all
all: build

.PHONY: build
build:
	docker build -t livepeerci/kube-nodes-connected .

.PHONY: push
push:
	docker push livepeerci/kube-nodes-connected
