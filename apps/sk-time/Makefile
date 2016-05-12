
.PHONY: app container push

all: app container clean

app: .
	npm install
	npm run-script build

container: .
	docker build -t gcr.io/stream-kitchen/sk-time .

push: .
	docker push gcr.io/stream-kitchen/sk-time

clean: .
	mkdir -p dist
	rm -rf dist
