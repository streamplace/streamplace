
.PHONY: app container push

all: app container clean

app: .
	npm install
	npm run-script build

container: .
	docker build -t streamkitchen/vertex-scheduler .

push: .
	docker push streamkitchen/vertex-scheduler

clean: .
	mkdir -p dist
	rm -rf dist
