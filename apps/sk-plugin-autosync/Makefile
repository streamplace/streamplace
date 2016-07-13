
.PHONY: app container push

all: app container clean

app: .
	npm install
	npm run-script build

container: .
	cd $(shell pwd)/.. && docker build -f sk-plugin-autosync/Dockerfile -t streamkitchen/sk-plugin-autosync .

push: .
	docker push streamkitchen/sk-plugin-autosync

clean: .
	mkdir -p dist
	rm -rf dist
