# Get the git tag from the current commit...
TAG=$(shell git describe --abbrev=0 --tags)
IMAGE=ghcr.io/tolson-vkn/pifrost
MAKEFILE_DIR=$(PWD)

.PHONY: help
help:
	@echo "+------------------+"
	@echo "| Makefile Targets |"
	@echo "+------------------+"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-12s\033[0m %s\n", $$1, $$2}'

.PHONY: build
build: ## Build Web.
	@echo "+--------------------+"
	@echo "| Building Container |"
	@echo "+--------------------+"
	docker build -t $(IMAGE) --build-arg version=$(shell git describe --abbrev=0) --build-arg gitcommit=$(shell git rev-parse HEAD) .

.PHONY: version
version: ## Make a release tag
	@echo "Tagging version."
	@# Get from https://github.com/tolson-vkn/forge/blob/master/dank-shell/semantic_version.sh
	@semantic_version.sh

.PHONY: publish
publish: ## Publish to GHCR
	@echo "Build and Publish"

	make version
	docker buildx build --push \
		--tag $(IMAGE):$(TAG) \
		--tag $(IMAGE):latest \
		--build-arg version=$(shell git describe --abbrev=0) \
		--build-arg gitcommit=$(shell git rev-parse HEAD) \
		--platform linux/amd64,linux/arm/v7,linux/arm64 .
