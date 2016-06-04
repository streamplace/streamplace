
.PHONY: app container push

all: app container clean

app: .
	npm install
	npm run-script build

container: .
	cd $(shell pwd)/.. && docker build -f bellamie/Dockerfile -t streamkitchen/bellamie .

push: .
	docker push streamkitchen/bellamie

clean: .
	mkdir -p dist
	rm -rf dist
