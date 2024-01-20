ifeq (, $(shell which staticcheck))
$(error "No staticcheck in $(PATH), consider doing: go get -u honnef.co/go/tools/cmd/staticcheck")
endif

ifeq (, $(shell which docker))
$(error "No docker in $(PATH))
endif

ifeq (, $(shell which kubectl))
$(error "No kubectl in $(PATH))
endif

DOCKER_IMAGE ?= aslan-spock-register.qiniu.io/qa/reviewbot
VERSION ?= latest

default: all

all: test fmt lint build

test:
	go test -v ./...
fmt:
	go fmt ./...
lint:
	staticcheck ./...	

build:
	go build .

docker-build: build
	docker builder build --push -t $(DOCKER_IMAGE):$(VERSION) .

docker-deploy: docker-build
	kubectl apply -k .