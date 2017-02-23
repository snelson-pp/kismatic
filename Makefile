# Set the build version
ifeq ($(origin VERSION), undefined)
	VERSION := $(shell git describe --tags --always --dirty)
endif
# build date
ifeq ($(origin BUILD_DATE), undefined)
	BUILD_DATE := $(shell date -u)
endif

# Setup some useful vars
PKG = github.com/apprenda/kismatic
HOST_GOOS = $(shell go env GOOS)
HOST_GOARCH = $(shell go env GOARCH)

# Versions of external dependencies
GLIDE_VERSION = v0.11.1
ANSIBLE_VERSION = 2.1.4.0
PROVISIONER_VERSION = v1.1
GO_VERSION = 1.8.0

ifeq ($(origin GLIDE_GOOS), undefined)
	GLIDE_GOOS := $(HOST_GOOS)
endif
ifeq ($(origin GOOS), undefined)
	GOOS := $(HOST_GOOS)
endif

build: bin/$(GOOS)/kismatic

build-inspector:
	@$(MAKE) GOOS=linux bin/inspector/linux/amd64/kismatic-inspector
	@$(MAKE) GOOS=darwin bin/inspector/darwin/amd64/kismatic-inspector

.PHONY: bin/$(GOOS)/kismatic
bin/$(GOOS)/kismatic: vendor
	@echo "building $@"
	@docker run                                                                     \
	    --rm                                                                        \
	    -e GOOS=$(GOOS)                                                             \
	    -u $$(id -u):$$(id -g)                                                      \
	    -v "$(shell pwd)":/go/src/$(PKG)                                            \
	    -w /go/src/$(PKG)                                                           \
	    golang:$(GO_VERSION)                                                        \
	    go build -o $@                                                              \
	        -ldflags "-X main.version=$(VERSION) -X 'main.buildDate=$(BUILD_DATE)'" \
	        ./cmd/kismatic

.PHONY: bin/inspector/$(GOOS)/kismatic-inspector
bin/inspector/$(GOOS)/amd64/kismatic-inspector: vendor
	@echo "building $@"
	@docker run                                                                      \
	    --rm                                                                         \
	    -e GOOS=$(GOOS)                                                              \
	    -u $$(id -u):$$(id -g)                                                       \
	    -v "$(shell pwd)":/go/src/$(PKG)                                             \
	    -w /go/src/$(PKG)                                                            \
	    golang:$(GO_VERSION)                                                         \
	    go build -o $@                                                               \
	        -ldflags "-X main.version=$(VERSION) -X 'main.buildDate=$(BUILD_DATE)'"  \
	        ./cmd/kismatic-inspector


clean:
	rm -rf bin
	rm -rf out
	rm -rf vendor
	rm -rf vendor-ansible/out
	rm -rf vendor-provision/out
	rm -rf integration/vendor

test: vendor
	@docker run                                                   \
	    --rm                                                      \
	    -u $$(id -u):$$(id -g)                                    \
	    -v "$(shell pwd)":/go/src/$(PKG)                          \
	    -w /go/src/$(PKG)                                         \
	    golang:$(GO_VERSION)                                      \
	    go test ./cmd/... ./pkg/... $(TEST_OPTS)

integration-test: dist just-integration-test

vendor: tools/glide
	./tools/glide install

tools/glide:
	mkdir -p tools
	curl -L https://github.com/Masterminds/glide/releases/download/$(GLIDE_VERSION)/glide-$(GLIDE_VERSION)-$(GLIDE_GOOS)-$(HOST_GOARCH).tar.gz | tar -xz -C tools
	mv tools/$(GLIDE_GOOS)-$(HOST_GOARCH)/glide tools/glide
	rm -r tools/$(GLIDE_GOOS)-$(HOST_GOARCH)

vendor-ansible/out:
	@echo "Vendoring ansible"
	@docker build -t apprenda/vendor-ansible vendor-ansible
	@docker run \
	    --rm \
	    -v $(shell pwd)/vendor-ansible/out:/ansible \
	    apprenda/vendor-ansible \
	    pip install --install-option="--prefix=/ansible" ansible==$(ANSIBLE_VERSION)

vendor-provision/out:
	mkdir -p vendor-provision/out/
	curl -L https://github.com/apprenda/kismatic-provision/releases/download/$(PROVISIONER_VERSION)/provision-darwin-amd64 -o vendor-provision/out/provision-darwin-amd64
	curl -L https://github.com/apprenda/kismatic-provision/releases/download/$(PROVISIONER_VERSION)/provision-linux-amd64 -o vendor-provision/out/provision-linux-amd64
	chmod +x vendor-provision/out/*

dist: vendor-ansible/out vendor-provision/out build build-inspector
	mkdir -p out
	cp bin/$(GOOS)/kismatic out
	mkdir -p out/ansible
	cp -r vendor-ansible/out/* out/ansible
	rm -rf out/ansible/playbooks
	cp -r ansible out/ansible/playbooks
	mkdir -p out/ansible/playbooks/inspector
	cp -r bin/inspector/* out/ansible/playbooks/inspector
	mkdir -p out/ansible/playbooks/kuberang/linux/amd64/
	curl https://kismatic-installer.s3-accelerate.amazonaws.com/latest/kuberang -o out/ansible/playbooks/kuberang/linux/amd64/kuberang
	cp vendor-provision/out/provision-$(GOOS)-amd64 out/provision
	rm -f out/kismatic.tar.gz
	tar -czf kismatic.tar.gz -C out .
	mv kismatic.tar.gz out

integration/vendor: tools/glide
	go get github.com/onsi/ginkgo/ginkgo
	cd integration && ../tools/glide install

just-integration-test: integration/vendor
	ginkgo --skip "\[slow\]" -p -v integration
	ginkgo --focus "\[slow\]" -p -v integration

serial-integration-test: integration/vendor
	ginkgo -v integration

focus-integration-test: integration/vendor
	ginkgo --focus $(FOCUS) -v integration

docs/kismatic-cli:
	mkdir docs/kismatic-cli
	go run cmd/kismatic-docs/main.go
	cp docs/kismatic-cli/kismatic.md docs/kismatic-cli/README.md

version: FORCE
	@echo VERSION=$(VERSION)
	@echo GLIDE_VERSION=$(GLIDE_VERSION)
	@echo ANSIBLE_VERSION=$(ANSIBLE_VERSION)
	@echo PROVISIONER_VERSION=$(PROVISIONER_VERSION)

FORCE:
