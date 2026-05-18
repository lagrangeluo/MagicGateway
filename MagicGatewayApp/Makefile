.PHONY: dev dev-watch build build-linux package clean

GO_IMAGE ?= golang:1.25-alpine
CONFIG   ?= config.test.yaml
VERSION  := $(shell cat VERSION)

dev:
	docker run --rm -p 8080:8080 \
	  -v $(PWD):/app -w /app \
	  -v $(PWD)/.go-cache:/go \
	  -e MAGIC_API_KEY -e JWT_SECRET \
	  $(GO_IMAGE) sh -c "go run -ldflags='-X main.Version=$(VERSION)' . $(CONFIG)"

dev-watch:
	docker run --rm -p 8080:8080 \
	  -v $(PWD):/app -w /app \
	  -v $(PWD)/.go-cache:/go \
	  -e MAGIC_API_KEY -e JWT_SECRET \
	  $(GO_IMAGE) sh -c \
	  "which air >/dev/null 2>&1 || go install github.com/air-verse/air@latest; air -- -ldflags '-X main.Version=$(VERSION)'"

build:
	docker build --build-arg VERSION=$(VERSION) -t magicgateway:latest .

build-linux:
	docker run --rm -v $(PWD):/app -w /app golang:1.25-alpine sh -c "CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags='-s -w -X main.Version=$(VERSION)' -o build/gateway ."

package: build-linux
	tar czf build/magicgateway.tar.gz -C build gateway -C . deploy/magicgateway.service config.yaml

clean:
	rm -rf build
