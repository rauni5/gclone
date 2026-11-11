# Common tasks. Everything here is plain `go`; nothing else is required.
VERSION ?= dev

.PHONY: build test vet fmt check release clean

build:
	go build -ldflags "-X main.version=$(VERSION)" -o gclone .

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -w .

# check is what CI runs: formatting, vet, tests.
check:
	@test -z "$$(gofmt -l .)" || (echo "gofmt needed on:"; gofmt -l .; exit 1)
	go vet ./...
	go test ./...

release:
	scripts/build-release.sh v$(VERSION)

clean:
	rm -rf gclone gclone.exe dist
