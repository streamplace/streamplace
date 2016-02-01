
all: .
	npm install
	npm run-script build
	docker build -v (shell pwd)/..:/lib -t gcr.io/dandiprat-stuff/bellamie .

app:
	

push: .
	docker push gcr.io/dandiprat-stuff/bellamie


