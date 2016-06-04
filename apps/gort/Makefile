
.PHONY: all app container push

all: app container

app: .
	npm install
	npm run-script build

container: .
	docker build -t streamkitchen/gort .

push: .
	docker push streamkitchen/gort
