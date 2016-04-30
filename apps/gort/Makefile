
.PHONY: all app container push

all: app container

app: .
	npm install
	npm run-script build

container: .
	docker build -t gcr.io/stream-kitchen/gort .

push: .
	docker push gcr.io/stream-kitchen/gort
