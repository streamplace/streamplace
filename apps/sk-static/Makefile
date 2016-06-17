
.PHONY: all container push

all: container

container: .
	cd $(shell pwd)/.. && docker build -f sk-static/Dockerfile -t streamkitchen/sk-static .

push: .
	docker push streamkitchen/sk-static
