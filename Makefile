.PHONY: all test testv clean fmt
.PHONY: swaprouter

GOBIN = ./build/bin
GOCMD = env GO111MODULE=on GOPROXY=https://goproxy.io,direct go

swaprouter:
	$(GOCMD) run build/ci.go install ./cmd/swaprouter
	@echo "Done building."
	@echo "Run \"$(GOBIN)/swaprouter\" to launch swaprouter."

all:
	$(GOCMD) build -v ./...
	$(GOCMD) run build/ci.go install ./cmd/...
	@echo "Done building."
	@echo "Find binaries in \"$(GOBIN)\" directory."
	@echo ""
	@echo "Copy example config files to \"$(GOBIN)\" directory"
	@cp -uv params/config*-example.toml $(GOBIN)

test: all
	$(GOCMD) test ./...

testv: all
	$(GOCMD) test -v ./...

clean:
	$(GOCMD) clean -cache
	rm -fr $(GOBIN)/*

fmt:
	./gofmt.sh
