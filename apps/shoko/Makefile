
.PHONY: container push

all: container

container: .
	docker build -t gcr.io/stream-kitchen/shoko .

push:
	docker push gcr.io/stream-kitchen/shoko
