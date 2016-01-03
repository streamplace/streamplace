
all: .
	npm install
	npm run-script build
	docker build -t gcr.io/dandiprat-stuff/bellamie-server .

push: .
	docker push gcr.io/dandiprat-stuff/bellamie-server
