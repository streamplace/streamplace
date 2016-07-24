
.PHONY: all container push

all: container

container: .
	docker build -t streamkitchen/sk-static .

push: .
	docker push streamkitchen/sk-static
