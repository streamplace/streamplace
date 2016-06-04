
.PHONY: container push

all: container

container: .
	docker build -t streamkitchen/shoko .

push:
	docker push streamkitchen/shoko
