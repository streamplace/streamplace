
.PHONY: app container push

all: app container clean

app: .
	npm install
	npm run-script build

container: .
	cd $(shell pwd)/.. && docker build -f autosync/Dockerfile -t streamkitchen/autosync .

push: .
	docker push streamkitchen/autosync

clean: .
	mkdir -p dist
	rm -rf dist
