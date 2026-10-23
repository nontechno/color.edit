GO ?= go
BINARY ?= color.edit

.PHONY: all build test vet run clean

all: test build

build:
	CGO_ENABLED=0 GOFLAGS=-buildvcs=false $(GO) build -trimpath -o $(BINARY) .

test:
	CGO_ENABLED=0 GOFLAGS=-buildvcs=false $(GO) test ./...

vet:
	CGO_ENABLED=0 GOFLAGS=-buildvcs=false $(GO) vet ./...

run: build
	./$(BINARY) --spec cmap --colors light.json

clean:
	$(GO) clean
	$(RM) $(BINARY)
