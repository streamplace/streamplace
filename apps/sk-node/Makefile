
.PHONY: all container push

container: .
	cd $(shell pwd)/.. && docker build -f sk-node/Dockerfile -t streamkitchen/sk-node .

push:
	docker push streamkitchen/sk-node
