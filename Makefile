.PHONY: run build tidy

run:
	go run .

build:
	go build -o bin/app .

tidy:
	go mod tidy
