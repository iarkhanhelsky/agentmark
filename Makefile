.PHONY: build run test vet fmt clean

GO      ?= go
BINARY  ?= agentmark
PORT    ?= 4173
MD      ?= notes.md

build:
	$(GO) build -o $(BINARY) .

run: build
	./$(BINARY) --port=$(PORT) $(MD)

test:
	$(GO) test ./... -count=1

vet:
	$(GO) vet ./...

fmt:
	$(GO) fmt ./...

clean:
	rm -f $(BINARY)
