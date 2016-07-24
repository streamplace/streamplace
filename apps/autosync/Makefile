
.PHONY: app container push

all: app container clean

app: .
	npm install
	npm run-script build

container: .
	docker build -t streamkitchen/autosync .

push: .
	docker push streamkitchen/autosync

clean: .
	mkdir -p dist
	rm -rf dist
