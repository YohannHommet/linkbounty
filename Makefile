BINARY = linkbounty
GO     = go
GOFLAGS = -trimpath

.PHONY: build test run clean

build:
	$(GO) build $(GOFLAGS) -o $(BINARY) ./cmd/web

test:
	$(GO) test ./...

run: build
	UI_DIR=./ui/html ./$(BINARY)

clean:
	rm -f $(BINARY) *.db *.db-wal *.db-shm
