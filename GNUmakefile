default: build

.PHONY: build
build:
	go build ./...

.PHONY: install
install:
	go install ./...

.PHONY: test
test:
	go test ./... -timeout=120s

.PHONY: testacc
testacc:
	TF_ACC=1 go test ./... -v -timeout=120m

.PHONY: fmt
fmt:
	gofmt -s -w -e .

.PHONY: vet
vet:
	go vet ./...
