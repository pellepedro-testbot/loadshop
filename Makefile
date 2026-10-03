# loadshop build targets. Requires Go 1.24+ and Node 20+.
BIN      := bin
LDFLAGS  := -s -w
GOFLAGS  := -trimpath

.PHONY: all web build linux test vet run loadgen docker clean ensure-dist

all: web build

web: web/node_modules
	cd web && npm run build

web/node_modules: web/package.json
	cd web && npm ci || npm install
	@touch web/node_modules

# go:embed needs web/dist to exist; a placeholder lets tests run without a UI build.
ensure-dist:
	@test -f web/dist/index.html || (mkdir -p web/dist && echo '<!doctype html><title>LoadShop</title><p>UI not built: run make web</p>' > web/dist/index.html)

build: ensure-dist
	go build $(GOFLAGS) -ldflags '$(LDFLAGS)' -o $(BIN)/loadshop .

linux: ensure-dist
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build $(GOFLAGS) -ldflags '$(LDFLAGS)' -o $(BIN)/loadshop-linux-amd64 .

loadgen:
	go build $(GOFLAGS) -o $(BIN)/loadgen ./tools/loadgen
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build $(GOFLAGS) -ldflags '$(LDFLAGS)' -o $(BIN)/loadgen-linux-amd64 ./tools/loadgen

vet: ensure-dist
	go vet ./...

test: ensure-dist
	go vet ./...
	go test -race ./...

run: build
	./$(BIN)/loadshop

docker: linux
	docker build --platform linux/amd64 -t loadshop:latest .

clean:
	rm -rf $(BIN) web/dist
