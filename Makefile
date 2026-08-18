ZIG := zig

export CGO_ENABLED=1
export CC=$(ZIG) cc
export CXX=$(ZIG) c++

.PHONY: build test vet run tidy

build:
	go build -o bin/verti ./cmd/verti

test:
	go test ./...

vet:
	go vet ./...

run: build
	./bin/verti

tidy:
	go mod tidy
