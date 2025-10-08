# SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company
# SPDX-License-Identifier: Apache-2.0

MAKEFLAGS=--warn-undefined-variables

# /bin/sh is dash on Debian which does not support all features of ash/bash
# to fix that we use /bin/bash only on Debian to not break Alpine
ifneq (,$(wildcard /etc/os-release)) # check file existence
	ifneq ($(shell grep -c debian /etc/os-release),0)
		SHELL := /bin/bash
	endif
endif

default: build-all

##################################################################################
# INSTALL
###################################################################################

install-goimports: FORCE
	@if ! hash goimports 2>/dev/null; then printf "\e[1;36m>> Installing goimports (this may take a while)...\e[0m\n"; go install golang.org/x/tools/cmd/goimports@latest; fi

install-golangci-lint: FORCE
	@if ! hash golangci-lint 2>/dev/null; then printf "\e[1;36m>> Installing golangci-lint (this may take a while)...\e[0m\n"; go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest; fi

install-modernize: FORCE
	@if ! hash modernize 2>/dev/null; then printf "\e[1;36m>> Installing modernize (this may take a while)...\e[0m\n"; go install golang.org/x/tools/gopls/internal/analysis/modernize/cmd/modernize@latest; fi

prepare-static-check: FORCE install-golangci-lint install-modernize

install-ginkgo: FORCE
	@if ! hash ginkgo 2>/dev/null; then printf "\e[1;36m>> Installing ginkgo (this may take a while)...\e[0m\n"; go install github.com/onsi/ginkgo/v2/ginkgo@latest; fi

##################################################################################
# BUILD
###################################################################################

GO_BUILDFLAGS =
GO_LDFLAGS =
GO_TESTENV =
GO_BUILDENV =

build-all: build/webhook

build/webhook: FORCE
	env $(GO_BUILDENV) go build $(GO_BUILDFLAGS) -ldflags '-s -w $(GO_LDFLAGS)' -o build/webhook ./cmd/webhook

# which packages to test with test runner
GO_TESTPKGS := $(shell go list -f '{{if or .TestGoFiles .XTestGoFiles}}{{.Dir}}{{end}}' ./...)
ifeq ($(GO_TESTPKGS),)
GO_TESTPKGS := ./...
endif
# which packages to measure coverage for
GO_COVERPKGS := $(shell go list ./...)
# to get around weird Makefile syntax restrictions, we need variables containing nothing, a space and comma
null :=
space := $(null) $(null)
comma := ,

check: FORCE static-check build/cover.html build-all
	@printf "\e[1;32m>> All checks successful.\e[0m\n"

run-golangci-lint: FORCE install-golangci-lint
	@printf "\e[1;36m>> golangci-lint\e[0m\n"
	@golangci-lint config verify
	@golangci-lint run

run-modernize: FORCE install-modernize
	@printf "\e[1;36m>> modernize\e[0m\n"
	@modernize $(GO_TESTPKGS)

build/cover.out: FORCE install-ginkgo | build
	@printf "\e[1;36m>> Running tests\e[0m\n"
	@env $(GO_TESTENV) ginkgo run --randomize-all -output-dir=build $(GO_BUILDFLAGS) -ldflags '-s -w $(GO_LDFLAGS)' -covermode=count -coverpkg=$(subst $(space),$(comma),$(GO_COVERPKGS)) $(GO_TESTPKGS)
	@mv build/coverprofile.out build/cover.out

build/cover.html: build/cover.out
	@printf "\e[1;36m>> go tool cover > build/cover.html\e[0m\n"
	@go tool cover -html $< -o $@

static-check: FORCE run-golangci-lint run-modernize

build:
	@mkdir $@

tidy-deps: FORCE
	go mod tidy
	go mod verify

goimports: FORCE install-goimports
	@printf "\e[1;36m>> goimports -w -local https://github.com/PlusOne/resource-name\e[0m\n"
	@goimports -w -local github.com/cloudoperators/common-cloud-resource-names $(patsubst $(shell awk '$$1 == "module" {print $$2}' go.mod)%,.%/*.go,$(shell go list ./...))

modernize: FORCE install-modernize
	@printf "\e[1;36m>> modernize -fix ./...\e[0m\n"
	@modernize -fix ./...

clean: FORCE
	git clean -dxf build

vars: FORCE
	@printf "GO_BUILDENV=$(GO_BUILDENV)\n"
	@printf "GO_BUILDFLAGS=$(GO_BUILDFLAGS)\n"
	@printf "GO_COVERPKGS=$(GO_COVERPKGS)\n"
	@printf "GO_LDFLAGS=$(GO_LDFLAGS)\n"
	@printf "GO_TESTENV=$(GO_TESTENV)\n"
	@printf "GO_TESTPKGS=$(GO_TESTPKGS)\n"
help: FORCE
	@printf "\n"
	@printf "\e[1mUsage:\e[0m\n"
	@printf "  make \e[36m<target>\e[0m\n"
	@printf "\n"
	@printf "\e[1mGeneral\e[0m\n"
	@printf "  \e[36mvars\e[0m                   Display values of relevant Makefile variables.\n"
	@printf "  \e[36mhelp\e[0m                   Display this help.\n"
	@printf "\n"
	@printf "\e[1mPrepare\e[0m\n"
	@printf "  \e[36minstall-goimports\e[0m      Install goimports required by goimports/static-check\n"
	@printf "  \e[36minstall-golangci-lint\e[0m  Install golangci-lint required by run-golangci-lint/static-check\n"
	@printf "  \e[36minstall-modernize\e[0m      Install modernize required by modernize/static-check\n"
	@printf "  \e[36mprepare-static-check\e[0m   Install any tools required by static-check. This is used in CI before dropping privileges, you should probably install all the tools using your package manager\n"
	@printf "  \e[36minstall-ginkgo\e[0m         Install ginkgo required when using it as test runner. This is used in CI before dropping privileges, you should probably install all the tools using your package manager\n"
	@printf "\n"
	@printf "\e[1mBuild\e[0m\n"
	@printf "  \e[36mbuild-all\e[0m              Build all binaries.\n"
	@printf "  \e[36mbuild/webhook\e[0m          Build webhook.\n"
	@printf "\n"
	@printf "\e[1mTest\e[0m\n"
	@printf "  \e[36mcheck\e[0m                  Run the test suite (unit tests and golangci-lint).\n"
	@printf "  \e[36mrun-golangci-lint\e[0m      Install and run golangci-lint. Installing is used in CI, but you should probably install golangci-lint using your package manager.\n"
	@printf "  \e[36mrun-modernize\e[0m          Install and run modernize. Installing is used in CI, but you should probably install modernize using your package manager.\n"
	@printf "  \e[36mbuild/cover.out\e[0m        Run tests and generate coverage report.\n"
	@printf "  \e[36mbuild/cover.html\e[0m       Generate an HTML file with source code annotations from the coverage report.\n"
	@printf "  \e[36mstatic-check\e[0m           Run static code checks\n"
	@printf "\n"
	@printf "\e[1mDevelopment\e[0m\n"
	@printf "  \e[36mtidy-deps\e[0m              Run go mod tidy and go mod verify.\n"
	@printf "  \e[36mgoimports\e[0m              Run goimports on all non-vendored .go files\n"
	@printf "  \e[36mmodernize\e[0m              Run modernize on all non-vendored .go files\n"
	@printf "  \e[36mclean\e[0m                  Run git clean.\n"

.PHONY: FORCE
