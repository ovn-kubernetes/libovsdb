OVS_VERSION ?= v2.16.0

.PHONY: all
all: lint build test integration-test coverage

.PHONY: modelgen
modelgen:
	@mkdir -p bin
	@go build -v -o ./bin ./cmd/modelgen

.PHONY: prebuild
prebuild: modelgen ovsdb/serverdb/_server.ovsschema example/vswitchd/ovs.ovsschema
	@echo "+ $@"
	@go generate -v ./...

.PHONY: build
build: prebuild 
	@echo "+ $@"
	@go build -v ./...

.PHONY: test
test: prebuild
	@echo "+ $@"
	@go test -race -coverprofile=unit.cov -test.short -timeout 30s -v ./...
	@IN_MEMORY_DATABASE=1 go test -race -coverprofile=ims-integration.cov -coverpkg=github.com/ovn-org/libovsdb/... -timeout 60s -v ./test/ovs

.PHONY: integration-test
integration-test:
	@echo "+ $@"
	@go test -race -coverprofile=integration.cov -coverpkg=github.com/ovn-org/libovsdb/... -timeout 60s -v ./test/ovs

.PHONY: coverage
coverage: test integration-test
	@sed -i '1d' integration.cov
	@sed -i '1d' ims-integration.cov
	@cat unit.cov integration.cov ims-integration.cov > profile.cov

.PHONY: bench
bench: install-deps
	@echo "+ $@"
	@go test -run=XXX -count=3 -bench=. ./... | tee bench.out
	@benchstat bench.out

.PHONY: install-deps
install-deps:
	@echo "+ $@"
	@golangci-lint --version
	@go install golang.org/x/perf/cmd/benchstat@latest

.PHONY: lint
lint: install-deps prebuild
	@echo "+ $@"
	@golangci-lint run

ovsdb/serverdb/_server.ovsschema:
	@curl -sSL https://raw.githubusercontent.com/openvswitch/ovs/${OVS_VERSION}/ovsdb/_server.ovsschema -o $@

example/vswitchd/ovs.ovsschema:
	@curl -sSL https://raw.githubusercontent.com/openvswitch/ovs/${OVS_VERSION}/vswitchd/vswitch.ovsschema -o $@