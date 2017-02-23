
.PHONY: test build

all: build

build: .
	npm install
	npm run build

test: .
	npm run test
	docker run -d --name rethink-test -p 28019:28015 rethinkdb:2.3 \
		&& npm run test-rethink \
		; docker rm -f rethink-test

push: .
	echo "nothing to do"
