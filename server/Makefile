build:
	GOOS=linux GOARCH=amd64	go build -o service_linux main.go

copy-to-server:
	scp service_linux root@46.148.238.155

docker-build-and-push:
	docker buildx build --no-cache --platform linux/amd64 -t <REGESTRY>/test-server:v0.0.1 .
	docker login -u <USERNAME> -p <PASSWORD> <REGESTRY>
	docker push <REGESTRY>/test-server:v0.0.1
	
run:
	go run main.go