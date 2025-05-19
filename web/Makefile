.DEFAULT_GOAL := build
TAG ?= latest
HELM_IMG ?= samirtahir91076/kube-jit-web
IMG ?= samirtahir91076/kube-jit-web:${TAG}

help:
		@echo ""
		@echo "=================="
		@echo ""
		@echo "Available commands :"
		@echo ""
		@echo "  make build \t\tBuilds image"
		@echo ""

build:
		docker build -t ${IMG} .

push:
		docker push ${IMG}

run-docker:
		docker run -p 5173:80 ${IMG}

run:
	@cd kube-jit && yarn dev

local-test:
	@cd kube-jit && yarn local-test

test:
	@cd kube-jit && yarn test

lint:
	@cd kube-jit && yarn lint

build-local:
	@cd kube-jit && yarn build

template:
		helm template -n kube-jit-web kube-jit-web chart/kube-jit-web --create-namespace

deploy:
		helm install -n kube-jit-web kube-jit-web chart/kube-jit-web --create-namespace -f sample/helm-values.yaml --set image.repository=$(HELM_IMG) --set image.tag=$(TAG)

deploy-gke:
		helm install -n kube-jit-web kube-jit-web chart/kube-jit-web --create-namespace -f sample/gke-values.yaml --set image.repository=$(HELM_IMG) --set image.tag=$(TAG)

deploy-aks:
		helm install -n kube-jit-web kube-jit-web chart/kube-jit-web --create-namespace -f sample/aks-values.yaml --set image.repository=$(HELM_IMG) --set image.tag=$(TAG)

delete:
		helm delete -n kube-jit-web kube-jit-web || true

get:
		kubectl -n kube-jit-web get all
	
kind-load:
		kind load docker-image ${IMG} --name kubejit

e2e: delete build deploy

e2e-gke: build push delete deploy-gke

.PHONY: help build