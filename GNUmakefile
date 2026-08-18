default: fmt lint install generate

build:
	go build -v ./...

install: build
	go install -v ./...

lint:
	golangci-lint run
	golangci-lint run --build-tags e2e

generate:
	cd tools; go generate ./...

fmt:
	gofmt -s -w -e .

# Everything that needs nothing but Go. The acceptance tests and the harness that
# starts a deployment for them are behind the `e2e` build tag, so they are not
# compiled here and this needs no Docker and no Terraform CLI.
test:
	go test -v -cover -timeout=120s -parallel=10 ./...

# The acceptance suite. Both switches are required: -tags e2e compiles the harness
# and the acceptance tests in, TF_ACC is terraform-plugin-testing's own gate.
# Without the tag this runs the same tests as `make test` and no more.
testacc:
	TF_ACC=1 go test -v -tags e2e -cover -timeout 120m ./...

.PHONY: fmt lint test testacc build install generate
