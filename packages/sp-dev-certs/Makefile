
.PHONY: all push

all: .
	npm run build
	docker build -t streamplace/sp-dev-certs .

push: .
	docker push streamplace/sp-dev-certs
