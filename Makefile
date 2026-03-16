OVS_VERSION ?= v3.5.0

TESTS ?=

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

.PHONY: check-generated
check-generated: prebuild
	@echo "+ $@"
	@if ! git diff --quiet; then \
		echo "Error: generated files are out of date. Please run 'make prebuild' and commit the changes."; \
		git diff --stat; \
		exit 1; \
	fi

.PHONY: build
build: prebuild
	@echo "+ $@"
	@go build -v ./...

.PHONY: test
test: prebuild
	@echo "+ $@"
	@go test -count=1 -race -coverprofile=unit.cov -test.short -timeout 30s -v $(if $(TESTS),-run $(TESTS)) ./...

.PHONY: integration-test
integration-test:
	@echo "+ $@"
	@go test -count=1 -race -coverprofile=integration.cov -coverpkg=github.com/ovn-kubernetes/libovsdb/... -timeout 60s -v $(if $(TESTS),-run $(TESTS)) ./test/ovs

.PHONY: coverage
coverage: test integration-test
	@sed -i '1d' integration.cov
	@cat unit.cov integration.cov > profile.cov

.PHONY: bench
bench: install-deps prebuild
	@echo "+ $@"
	@go test -run=XXX -count=3 $(if $(TESTS),-bench $(TESTS),-bench .) ./... | tee bench.out
	@benchstat bench.out

.PHONY: install-deps
install-deps:
	@echo "+ $@"
	@./hack/install-deps.sh

.PHONY: lint
lint: install-deps prebuild
	@echo "+ $@"
	@golangci-lint run

ovsdb/serverdb/_server.ovsschema:
	@curl -sSL https://raw.githubusercontent.com/openvswitch/ovs/${OVS_VERSION}/ovsdb/_server.ovsschema -o $@

example/vswitchd/ovs.ovsschema:
	@curl -sSL https://raw.githubusercontent.com/openvswitch/ovs/${OVS_VERSION}/vswitchd/vswitch.ovsschema -o $@
