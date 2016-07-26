
SK_DOCKER_APP = sk-node
SK_DOCKER_REPO ?= docker.io/streamkitchen
SK_DOCKER_VERSION ?= latest
SK_DOCKER_TAG ?= $(SK_DOCKER_REPO)/$(SK_DOCKER_APP):$(SK_DOCKER_VERSION)

.PHONY: container push clean

all: container clean

container: .
	cd $(shell pwd)/.. && docker build -f sk-node/Dockerfile -t streamkitchen/sk-node .
	docker tag -f streamkitchen/$(SK_DOCKER_APP) $(SK_DOCKER_TAG)

push: .
	docker push $(SK_DOCKER_TAG)

clean: .
	rm -rf dist
