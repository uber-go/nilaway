.PHONY: build
build:
	go build ./...

.PHONY: test
test:
	go test -race ./...

.PHONY: cover
cover:
	go test -v -race -coverprofile=cover.out -coverpkg=./... -v ./...
	go tool cover -html=cover.out -o cover.html
