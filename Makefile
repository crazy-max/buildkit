ifneq (, $(BUILDX_BIN))
	export BUILDX_CMD = $(BUILDX_BIN)
else ifneq (, $(shell docker buildx version))
	export BUILDX_CMD = docker buildx
else ifneq (, $(shell which buildx))
	export BUILDX_CMD = $(which buildx)
else
	export BUILDX_CMD = docker buildx
endif

prefix=/usr/local
bindir=$(prefix)/bin

.PHONY: binaries
binaries:
	hack/binaries

.PHONY: images
images:
# moby/buildkit:local and moby/buildkit:local-rootless are created on Docker
	hack/images local moby/buildkit
	TARGET=rootless hack/images local moby/buildkit

.PHONY: install
install:
	mkdir -p $(DESTDIR)$(bindir)
	install bin/* $(DESTDIR)$(bindir)

.PHONY: clean
clean:
	rm -rf ./bin

.PHONY: test
test:
	hack/test integration gateway dockerfile

.PHONY: lint
lint:
	$(BUILDX_CMD) bake lint

.PHONY: validate-vendor
validate-vendor:
	$(BUILDX_CMD) bake validate-vendor

.PHONY: validate-shfmt
validate-shfmt:
	hack/validate-shfmt

.PHONY: shfmt
shfmt:
	hack/shfmt

.PHONY: validate-authors
validate-authors:
	$(BUILDX_CMD) bake validate-authors

.PHONY: validate-generated-files
validate-generated-files:
	hack/validate-generated-files

.PHONY: validate-all
validate-all: test lint validate-vendor validate-generated-files

.PHONY: vendor
vendor:
	hack/update-vendor

.PHONY: authors
authors:
	$(BUILDX_CMD) bake authors

.PHONY: generated-files
generated-files:
	./hack/update-generated-files
