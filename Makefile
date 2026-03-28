WAILS    := $(HOME)/go/bin/wails
VCSIM    := ./build/bin/vcsim
VCSIM_PID := /tmp/xman-vcsim.pid
GO_TEST_ENV := GOCACHE=/tmp/gocache GOTMPDIR=/tmp
GO_TEST := $(GO_TEST_ENV) go test

.PHONY: dev build build-windows deploy-windows clean tidy vet test test-go test-go-cached test-vcenter test-vcenter-docker test-workstation test-workstation-integration test-frontend test-all test-all-cached vcsim-build vcsim vcsim-bg vcsim-stop doctor help

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
	cp ./build/bin/xman.exe /mnt/c/Users/yisra/Desktop/

## clean: remove build output
clean:
	rm -rf build/bin

## tidy: tidy go modules
tidy:
	go mod tidy

## vet: run go vet across all packages
vet:
	go vet ./...

## test: run the default backend test suite
test:
	$(MAKE) test-go

## test-go: run all Go tests with isolated temp/cache dirs
test-go:
	$(GO_TEST) -count=1 ./...

## test-go-cached: run all Go tests with Go's normal test cache enabled
test-go-cached:
	$(GO_TEST) ./...

## test-vcenter: run only the vcsim-backed vCenter backend tests
test-vcenter:
	$(GO_TEST) -count=1 ./internal/vcenter

## test-vcenter-docker: run opt-in docker-backed vCenter guest-ops integration tests
## requires: docker available from Linux/WSL and pulls/runs debian:stable-slim for container-backed vcsim VMs
test-vcenter-docker:
	XMAN_DOCKER_GUESTOPS=1 $(GO_TEST) -count=1 ./internal/vcenter -run DockerGuestOps

## test-workstation: run Workstation backend unit and fake-vmrun tests
test-workstation:
	$(GO_TEST) -count=1 ./internal/workstation

## test-workstation-integration: run opt-in real Workstation tests from WSL/host setup
## requires: XMAN_WS_INTEGRATION=1 XMAN_WS_VMRUN=<path> XMAN_WS_VM_DIR=<dir>
test-workstation-integration:
	XMAN_WS_INTEGRATION=1 $(GO_TEST) -count=1 ./internal/workstation -run WorkstationIntegration

## test-frontend: run the frontend production build as a validation check
test-frontend:
	cd frontend && npm run build

## test-all: run backend tests plus frontend validation
test-all: test-go test-frontend

## test-all-cached: run cached Go tests plus frontend validation
test-all-cached: test-go-cached test-frontend

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
