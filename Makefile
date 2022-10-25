
.PHONY: download-spec-tests
download-spec-tests:
	git submodule init
	git submodule update

.PHONY: bindata
bindata:
	go-bindata -pkg chain -o ./chain/chain_bindata.go ./chain/chains

.PHONY: protoc
protoc:
	protoc --go_out=. --go-grpc_out=. ./server/proto/*.proto
	protoc --go_out=. --go-grpc_out=. ./protocol/proto/*.proto
	protoc --go_out=. --go-grpc_out=. ./network/proto/*.proto
	protoc --go_out=. --go-grpc_out=. ./txpool/proto/*.proto
	protoc --go_out=. --go-grpc_out=. ./consensus/ibft/**/*.proto

.PHONY: build
build:
	$(eval LATEST_VERSION = $(shell git describe --tags --abbrev=0))
	$(eval COMMIT_HASH = $(shell git rev-parse HEAD))
	$(eval DATE = $(shell date -u +'%Y-%m-%dT%TZ'))
	go build -o dogechain -ldflags="\
		-X 'github.com/dogechain-lab/dogechain/versioning.Version=$(LATEST_VERSION)'\
		-X 'github.com/dogechain-lab/dogechain/versioning.Commit=$(COMMIT_HASH)'\
		-X 'github.com/dogechain-lab/dogechain/versioning.BuildTime=$(DATE)'" \
	main.go

.PHONY: lint
lint:
	golangci-lint run -c .golangci.yml

.PHONY: test
test: build
	PATH=$(shell pwd):${PATH} go test -count=1 -coverprofile coverage.out -timeout 28m ./...

.PHONY: test-e2e
test-e2e: build
    # We need to build the binary with the race flag enabled
    # because it will get picked up and run during e2e tests
    # and the e2e tests should error out if any kind of race is found
	go build -race -o artifacts/dogechain .
	PATH=${PWD}/artifacts:${PATH} go test -v -timeout=30m ./e2e/...

.PHONY: generate-bsd-licenses
generate-bsd-licenses:
	./generate_dependency_licenses.sh BSD-3-Clause,BSD-2-Clause > ./licenses/bsd_licenses.json
