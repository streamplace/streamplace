
SK_DOCKER_APP = broadcast-scheduler
SK_DOCKER_REPO ?= docker.io/streamkitchen
SK_DOCKER_VERSION ?= latest
SK_DOCKER_TAG ?= $(SK_DOCKER_REPO)/$(SK_DOCKER_APP):$(SK_DOCKER_VERSION)

.PHONY: app container push

all: app container clean

app: .
	npm install
	NODE_PATH=".." npm run-script build

container: .
	docker build -t streamkitchen/$(SK_DOCKER_APP) .
	docker tag streamkitchen/$(SK_DOCKER_APP) $(SK_DOCKER_TAG)

push: .
	docker push $(SK_DOCKER_TAG)

clean: .
	rm -rf dist
