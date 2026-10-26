version=$(shell git describe --always --long --dirty)
date=$(shell date "+(%b %Y)")
exec=mlgo

all:
	@go build -o $(exec) -v -ldflags '-X "main.version=$(version)" -X "main.date=$(date)"' ./cmd/mlgo

install:
	@go install -v -ldflags '-X "main.version=$(version)" -X "main.date=$(date)"' ./cmd/mlgo

test:
	@go test ./...

clean:
	@rm -f $(exec)
