
.PHONY: app container push run

all: app container clean

app: .
	npm install
	npm run-script build

container: .
	cd $(shell pwd)/.. && docker build -f pipeland/Dockerfile -t gcr.io/stream-kitchen/pipeland .

push: .
	docker push gcr.io/stream-kitchen/pipeland

clean: .
	mkdir -p dist
	rm -rf dist
