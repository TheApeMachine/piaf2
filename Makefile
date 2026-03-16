.PHONY: run install

run:
	go run main.go

install:
	go build .
	go install .