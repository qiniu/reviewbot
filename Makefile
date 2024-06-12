DOCKER_IMAGE ?= aslan-spock-register.qiniu.io/qa/reviewbot
VERSION ?= latest
TAG?=$(shell git describe --tag --always)
LDFLAGS=-X 'github.com/qiniu/reviewbot/internal/version.version=$(TAG)'

define check_command
	@if [ -z "$$(which $(1))" ]; then \
		echo "No $(1) in $(PATH), consider installing it."; \
		exit 1; \
	fi
endef

all: fmt vet staticcheck build test

check-go:
	$(call check_command,go)

check-docker:
	$(call check_command,docker)

check-kubectl:
	$(call check_command,kubectl)

check-staticcheck:
	$(call check_command,staticcheck)

test: check-go
	go test -v ./...
fmt: check-go
	go fmt ./...
vet: check-go
	go vet ./...		

staticcheck: check-staticcheck
	staticcheck ./...

build: check-go
	CGO_ENABLED=0 go build -v -trimpath -ldflags "$(LDFLAGS)" -o ./reviewbot .

docker-build: check-docker
	docker builder build --push -t $(DOCKER_IMAGE):$(VERSION) --target runner .


docker-dev: check-docker
	docker builder build -t $(DOCKER_IMAGE):dev --target runner .


docker-deploy: check-kubectl docker-build
	kubectl apply -k .