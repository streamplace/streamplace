
.PHONY: app container push

all: app container clean

app: .
	npm install
	npm run-script build

container: .
	docker build -t streamkitchen/sk-time .

push: .
	docker push streamkitchen/sk-time

clean: .
	mkdir -p dist
	rm -rf dist
