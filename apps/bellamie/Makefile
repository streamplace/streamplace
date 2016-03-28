
.PHONY: app container push

all: app container clean

app: .
	npm install
	npm run-script build

container: .
	cd $(shell pwd)/.. && docker build -f bellamie/Dockerfile -t gcr.io/stream-kitchen/bellamie .

push: .
	docker push gcr.io/stream-kitchen/bellamie

clean: .
	mkdir -p dist
	rm -rf dist
