all: .
	docker run \
		--rm \
		-it \
		-e GOPATH=/usr/src/myapp/Godeps/_workspace \
		-v $(shell pwd):/usr/src/myapp \
		-w /usr/src/myapp golang:1.3 go build \
		-o controller controller.go
	docker build -t gcr.io/dandiprat-stuff/pratty-balancer .

push: .
	docker push gcr.io/dandiprat-stuff/pratty-balancer
