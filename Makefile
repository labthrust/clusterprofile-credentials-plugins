BINARY := eks-aws-auth-plugin
CMD := ./cmd/eks-aws-auth-plugin
BIN_DIR := bin
DIST_DIR := dist
GO := go

GORELEASER ?= goreleaser

OS ?= linux darwin windows
ARCH ?= amd64 arm64
EXT_WIN := .exe

# Release settings
REMOTE ?= origin
MSG ?= Release $(VERSION)

.PHONY: help build build-current snapshot clean checksum package

help:
	@echo "Available targets:"
	@echo "  build       - Build local binary into $(BIN_DIR)/$(BINARY)"
	@echo "  snapshot    - Cross-build with GoReleaser, archive, and checksums (no publish)"
	@echo "  checksum    - Generate SHA256 checksums for dist archives"
	@echo "  package     - Archive the locally built binary"
	@echo "  clean       - Remove $(BIN_DIR) and $(DIST_DIR)"
	@echo "  tag-push    - Create annotated git tag and push VERSION=vX.Y.Z"

build:
	@mkdir -p $(BIN_DIR)
	$(GO) build -trimpath -ldflags "-s -w" -o $(BIN_DIR)/$(BINARY) $(CMD)
	@echo "Built $(BIN_DIR)/$(BINARY)"

clean:
	rm -rf $(BIN_DIR) $(DIST_DIR)

# GoReleaser snapshot build (no publish)
.PHONY: snapshot
snapshot:
	@$(GORELEASER) release --snapshot --clean --skip=publish

# Create annotated tag-push VERSION=vX.Y.Z
.PHONY: tag-push
tag-push:
	@test -n "$(VERSION)" || { echo "VERSION is required, e.g. make tag VERSION=v1.2.3" >&2; exit 1; }
	@case "$(VERSION)" in v*) ;; *) echo "VERSION must start with 'v' (e.g. v1.2.3)" >&2; exit 1;; esac
	@# ensure clean working tree (no unstaged/staged changes)
	@git status --porcelain | grep -q . && { echo "Working tree not clean. Commit or stash changes first." >&2; exit 1; } || true
	@# prevent duplicate tag
	@(! git rev-parse -q --verify "refs/tags/$(VERSION)" >/dev/null) || { echo "Tag $(VERSION) already exists" >&2; exit 1; }
	@git tag -a "$(VERSION)" -m "$(MSG)"
	@echo "Created tag $(VERSION)"
	@git push "$(REMOTE)" "$(VERSION)"
	@echo "Pushed tag $(VERSION) to $(REMOTE)"

# Generate checksums for archives in dist
checksum:
	@set -e; \
	mkdir -p $(DIST_DIR); \
	cd $(DIST_DIR); \
	rm -f checksums.txt; \
	if command -v shasum >/dev/null 2>&1; then SHACMD="shasum -a 256"; \
	elif command -v sha256sum >/dev/null 2>&1; then SHACMD="sha256sum"; \
	else echo "No shasum/sha256sum found" >&2; exit 1; fi; \
	for f in *.tar.gz *.zip; do \
	  [ -f "$$f" ] || continue; \
	  $$SHACMD "$$f" >> checksums.txt; \
	done; \
	echo "Wrote $(DIST_DIR)/checksums.txt"

# Archive the locally built binary for current host OS/ARCH
package: build
	@set -e; \
	mkdir -p $(DIST_DIR); \
	EXT=""; \
	OS_NAME="$$(go env GOOS)"; \
	ARCH_NAME="$$(go env GOARCH)"; \
	if [ "$$OS_NAME" = "windows" ]; then EXT="$(EXT_WIN)"; fi; \
	SRC="$(BIN_DIR)/$(BINARY)$$EXT"; \
	if [ ! -f "$$SRC" ]; then echo "binary $$SRC not found. Run 'make build' first."; exit 1; fi; \
	if [ "$$OS_NAME" = "windows" ]; then \
	  ARCHIVE="$(DIST_DIR)/$(BINARY)-$$OS_NAME-$$ARCH_NAME.zip"; \
	  cp "$$SRC" "$(DIST_DIR)/"; \
	  (cd $(DIST_DIR) && zip -q "$$(basename $$ARCHIVE)" "$$(basename $$SRC)"); \
	else \
	  ARCHIVE="$(DIST_DIR)/$(BINARY)-$$OS_NAME-$$ARCH_NAME.tar.gz"; \
	  cp "$$SRC" "$(DIST_DIR)/"; \
	  (cd $(DIST_DIR) && tar -czf "$$ARCHIVE" "$$(basename $$SRC)"); \
	fi; \
	echo "Created $$ARCHIVE"
