
.PHONY: app container push

all: app container

app: .
	npm install
	npm run-script build

container: .
	cd $(shell pwd)/.. && docker build -f bellamie/Dockerfile -t gcr.io/stream-kitchen/bellamie .

push: .
	docker push gcr.io/dandiprat-stuff/bellamie


