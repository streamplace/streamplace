
.PHONY: app container push

all: app container clean

app: .
	npm install
	npm run-script build

container: .
	cd $(shell pwd)/.. && docker build -f vertex-scheduler/Dockerfile -t streamkitchen/vertex-scheduler .

push: .
	docker push streamkitchen/vertex-scheduler

clean: .
	mkdir -p dist
	rm -rf dist
