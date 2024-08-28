
GOPATH := $(shell go env GOPATH)

fmt:
	@(test -f "$(GOPATH)/bin/gofumpt" || go install mvdan.cc/gofumpt@latest) && \
	"$(GOPATH)/bin/gofumpt" -l -w .

test:
	@go test ./...