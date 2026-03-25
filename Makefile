WAILS    := $(HOME)/go/bin/wails
VCSIM    := ./build/bin/vcsim
VCSIM_PID := /tmp/manosphere-vcsim.pid

.PHONY: dev build build-windows deploy-windows clean tidy vet test vcsim-build vcsim vcsim-bg vcsim-stop doctor help

## help: list available targets
help:
	@grep -E '^## ' Makefile | sed 's/## //'

## dev: start the development server with live reload
dev:
	$(WAILS) dev

## build: build Linux binary
build:
	$(WAILS) build

## build-windows: cross-compile a Windows .exe from Linux/WSL2
build-windows:
	$(WAILS) build -platform windows/amd64

## deploy-windows: build Windows .exe and copy to Windows desktop
deploy-windows: build-windows
	cp ./build/bin/manosphere.exe /mnt/c/Users/yisra/Desktop/

## clean: remove build output
clean:
	rm -rf build/bin

## tidy: tidy go modules
tidy:
	go mod tidy

## vet: run go vet across all packages
vet:
	go vet ./...

## test: run all tests
test:
	go test ./...

## vcsim-build: build the vcsim test simulator binary
vcsim-build:
	go build -o $(VCSIM) ./cmd/vcsim

## vcsim: build and start vcsim in the foreground — Ctrl+C to stop
## options: DC=1 CLUSTER=1 HOST=3 VM=5 U=user P=pass
## connect: http://127.0.0.1:8989/sdk  skip-tls:false
vcsim: vcsim-build
	exec $(VCSIM) -dc $(or $(DC),1) -cluster $(or $(CLUSTER),1) -host $(or $(HOST),3) -vm $(or $(VM),5) -u $(or $(U),user) -p $(or $(P),pass)

## vcsim-stop: stop a background vcsim started with vcsim-bg
vcsim-stop:
	@if [ -f $(VCSIM_PID) ]; then \
		kill $$(cat $(VCSIM_PID)) && rm $(VCSIM_PID) && echo "vcsim stopped."; \
	else \
		echo "vcsim is not running."; \
	fi

## doctor: check Wails dependencies are satisfied
doctor:
	$(WAILS) doctor
