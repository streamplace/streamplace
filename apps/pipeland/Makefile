
SK_DOCKER_APP = pipeland
SK_DOCKER_REPO ?= docker.io/streamkitchen
SK_DOCKER_VERSION ?= latest
SK_DOCKER_TAG ?= $(SK_DOCKER_REPO)/$(SK_DOCKER_APP):$(SK_DOCKER_VERSION)

.PHONY: app container push clean dev

all: app container clean

app: .
	npm install
	NODE_PATH=".." npm run-script build

dev: .
	npm run-script build
	docker build -f Dockerfile.dev -t streamkitchen/pipeland-dev .

container: .
	cd $(shell pwd)/.. && docker build -f pipeland/Dockerfile -t streamkitchen/$(SK_DOCKER_APP) .
	docker tag -f streamkitchen/$(SK_DOCKER_APP) $(SK_DOCKER_TAG)

push: .
	docker push $(SK_DOCKER_TAG)

clean: .
	rm -rf dist
