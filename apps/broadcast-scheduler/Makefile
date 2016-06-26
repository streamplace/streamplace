
.PHONY: app container push

all: app container clean

app: .
	npm install
	npm run-script build

container: .
	cd $(shell pwd)/.. && docker build -f broadcast-scheduler/Dockerfile -t streamkitchen/broadcast-scheduler .

push: .
	docker push streamkitchen/broadcast-scheduler

clean: .
	mkdir -p dist
	rm -rf dist
