
.PHONY: app container push

all: app container clean

app: .
	npm install
	npm run-script build

container: .
	docker build -t streamkitchen/bellamie .

push: .
	docker push streamkitchen/bellamie

clean: .
	mkdir -p dist
	rm -rf dist
