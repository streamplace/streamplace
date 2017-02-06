
.PHONY: all push

all: .
	npm run build
	docker build -t streamplace/sp-compositor .

push: .
	docker push streamplace/sp-compositor
