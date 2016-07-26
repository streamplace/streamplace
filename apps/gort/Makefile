
SK_DOCKER_APP = gort
SK_DOCKER_REPO ?= docker.io/streamkitchen
SK_DOCKER_VERSION ?= latest
SK_DOCKER_TAG ?= $(SK_DOCKER_REPO)/$(SK_DOCKER_APP):$(SK_DOCKER_VERSION)

.PHONY: app container push

all: app container

app: .
	npm install
	NODE_PATH=".." npm run-script build

container: .
	docker build -t streamkitchen/$(SK_DOCKER_APP) .
	docker tag -f streamkitchen/$(SK_DOCKER_APP) $(SK_DOCKER_TAG)

push: .
	docker push $(SK_DOCKER_TAG)
