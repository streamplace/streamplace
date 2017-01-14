
SK_DOCKER_APP = sk-builder
SK_DOCKER_REPO ?= docker.io/streamkitchen
SK_DOCKER_VERSION ?= latest
SK_DOCKER_TAG ?= $(SK_DOCKER_REPO)/$(SK_DOCKER_APP):$(SK_DOCKER_VERSION)

.PHONY: container push

all: container clean

container: .
	docker build -t streamkitchen/$(SK_DOCKER_APP) .
	docker tag streamkitchen/$(SK_DOCKER_APP) $(SK_DOCKER_TAG)

push: .
	docker push $(SK_DOCKER_TAG)

clean: .
	rm -rf dist
