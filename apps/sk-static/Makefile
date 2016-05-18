
.PHONY: all musta container

all: musta container

musta: .
	mkdir -p bin
	docker run -v $(PWD)/bin:/go/bin golang go get github.com/streamkitchen/musta

container: .
	docker build -t gcr.io/stream-kitchen/sk-static .
