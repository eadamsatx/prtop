.PHONY: build run install clean fmt lint

build:
	go build -o prtop .

run: build
	./prtop $(ARGS)

install:
	go install .

clean:
	rm -f prtop

fmt:
	go fmt ./...

lint:
	go vet ./...
