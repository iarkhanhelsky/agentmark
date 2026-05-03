.PHONY: build run test vet fmt fmt-check ci clean

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

fmt-check:
	@out=$$(gofmt -l .); \
	if [ -n "$$out" ]; then \
		echo "not formatted (run make fmt):"; \
		echo "$$out"; \
		exit 1; \
	fi

ci: fmt-check vet test build

clean:
	rm -f $(BINARY)
