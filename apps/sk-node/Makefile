
.PHONY: all container push

container: .
	docker build -t gcr.io/stream-kitchen/sk-node .

push:
	docker push gcr.io/stream-kitchen/sk-node
