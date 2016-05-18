
.PHONY: all container push

all: container

container: .
	docker build -t gcr.io/stream-kitchen/sk-static .

push: .
	docker push gcr.io/stream-kitchen/sk-static
