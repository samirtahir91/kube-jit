.DEFAULT_GOAL := build
.tag		  := kube-jit-web:latest

help:
		@echo ""
		@echo "=================="
		@echo ""
		@echo "Available commands :"
		@echo ""
		@echo "  make build \t\tBuilds image"
		@echo ""

build:
		docker build -t $(.tag) .

run:
		docker run -p 5173:80 $(.tag)

template:
		helm template -n kube-jit-web kube-jit-web chart/kube-jit-web --create-namespace

deploy:
		helm install -n kube-jit-web kube-jit-web chart/kube-jit-web --create-namespace -f sample/helm-values.yaml

deploy-gke:
		helm install -n kube-jit-web kube-jit-web chart/kube-jit-web --create-namespace -f sample/gke-values.yaml

delete:
		helm delete -n kube-jit-web kube-jit-web || true

get:
		kubectl -n kube-jit-web get all
	
kind-load:
		kind load docker-image $(.tag) --name kubejit

e2e: delete build deploy

.PHONY: help build