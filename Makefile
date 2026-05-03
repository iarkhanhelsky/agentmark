.PHONY: build run test vet fmt clean build-darwin-arm64 build-linux-amd64 cross-compile

GO      ?= go
BINARY  ?= agentmark
PORT    ?= 4173
MD      ?= notes.md
DISTDIR ?= dist/agentmark

build:
	mkdir -p $(DISTDIR)
	$(GO) build -o $(DISTDIR)/$(BINARY) .

build-darwin-arm64:
	mkdir -p $(DISTDIR)
	GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 $(GO) build -o $(DISTDIR)/$(BINARY)-darwin-arm64 .

build-linux-amd64:
	mkdir -p $(DISTDIR)
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 $(GO) build -o $(DISTDIR)/$(BINARY)-linux-amd64 .

cross-compile: build-darwin-arm64 build-linux-amd64

run: build
	./$(BINARY) --port=$(PORT) $(MD)

test:
	$(GO) test ./... -count=1

vet:
	$(GO) vet ./...

fmt:
	$(GO) fmt ./...

clean:
	rm -rf $(DISTDIR)
