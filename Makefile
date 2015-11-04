# MAKEFILE
#
# @author      Nicola Asuni <nicola.asuni@datasift.com>
# @link        https://github.com/quipo/workerpoolmanager
# ------------------------------------------------------------------------------

# List special make targets that are not associated with files
.PHONY: help all test format fmtcheck vet lint coverage qa deps install uninstall clean nuke build rpm dist

# Ensure everyone is using bash. Note that Ubuntu now uses dash which doesn't support PIPESTATUS.
SHELL=/bin/bash

# Project version
VERSION=$(shell cat VERSION)

# Project release number (packaging build number)
RELEASE=$(shell cat RELEASE)

# name of RPM or DEB package
PKGNAME=workerpoolmanager

# Go lang path
GOPATH=$(shell readlink -f $(shell pwd)/../../../../)

# Binary path (where the executable files will be installed)
BINPATH=usr/bin/

# Configuration path
CONFIGPATH=etc/workerpoolmanager/

# Default installation path for documentation
DOCPATH=usr/share/doc/$(PKGNAME)/

# Installation path for the code
PATHINSTBIN=$(DESTDIR)/$(BINPATH)

# Installation path for the configuration files
PATHINSTCFG=$(DESTDIR)/$(CONFIGPATH)

# Installation path for documentation
PATHINSTDOC=$(DESTDIR)/$(DOCPATH)

# Current directory
CURRENTDIR=$(shell pwd)

# RPM Packaging path (where RPMs will be stored)
PATHRPMPKG=$(CURRENTDIR)/target/RPM


# --- MAKE TARGETS ---

# Display general help about this command
help:
	@echo ""
	@echo "Welcome to $(PKGNAME) make."
	@echo "The following commands are available:"
	@echo ""
	@echo "    make qa         : Run all the tests"
	@echo ""
	@echo "    make test       : Run the unit tests"
	@echo "    make test.short : Run the unit tests with the short option"
	@echo ""
	@echo "    make format     : Format the source code"
	@echo "    make fmtcheck   : Check if the source code has been formatted"
	@echo "    make vet        : Check for syntax errors"
	@echo "    make lint       : Check for style errors"
	@echo "    make coverage   : Generate the coverage report"
	@echo ""
	@echo "    make docs       : Generate source code documentation"
	@echo ""
	@echo "    make deps       : Get the dependencies"
	@echo "    make build      : Compile the application"
	@echo "    make clean      : Remove any build artifact"
	@echo "    make nuke       : Deletes any intermediate file"
	@echo "    make install    : Install this application"
	@echo ""
	@echo "    make rpm        : Build an RPM package for the current OS (only supports RedHat-like systems)"
	@echo "    make dist       : Execute all tests (qa) and build the RPM package"
	@echo ""

# Alias for help target
all: help

# Run the unit tests
test:
	@mkdir -p target/test
	GOPATH=$(GOPATH) go test -race -v ./... | tee >(PATH=$(GOPATH)/bin:$(PATH) go-junit-report > target/test/report.xml); test $${PIPESTATUS[0]} -eq 0

# Run the unit tests with the short option
test.short:
	@mkdir -p target/test
	GOPATH=$(GOPATH) go test -short -race -v ./... | tee >(PATH=$(GOPATH)/bin:$(PATH) go-junit-report > target/test/report.xml); test $${PIPESTATUS[0]} -eq 0

# Format the source code
format:
	@find ./ -type f -name "*.go" -exec gofmt -w {} \;

# Check if the source code has been formatted
fmtcheck:
	@mkdir -p target
	@find ./ -type f -name "*.go" -exec gofmt -d {} \; | tee target/format.diff
	@test ! -s target/format.diff || { echo "ERROR: the source code has not been formatted - please use 'make format' or 'gofmt'"; exit 1; }

# Check for syntax errors
vet:
	GOPATH=$(GOPATH) go vet ./...

# Check for style errors
lint:
	GOPATH=$(GOPATH) PATH=$(GOPATH)/bin:$(PATH) golint ./...

# Generate the coverage report
coverage:
	@mkdir -p target/report
	GOPATH=$(GOPATH) ./coverage.sh

# Generate source docs
docs:
	@mkdir -p target/docs
	nohup sh -c 'GOPATH=$(GOPATH) godoc -http=127.0.0.1:6060' > target/godoc_server.log 2>&1 &
	wget --directory-prefix=target/docs/ --execute robots=off --retry-connrefused --recursive --no-parent --adjust-extension --page-requisites --convert-links http://127.0.0.1:6060/pkg/github.com/datasift/workerpoolmanager/ ; kill -9 `lsof -ti :6060`
	echo '<html><head><meta http-equiv="refresh" content="0;./127.0.0.1:6060/pkg/github.com/datasift/'${PKGNAME}'/index.html"/></head><a href="./127.0.0.1:6060/pkg/github.com/datasift/'${PKGNAME}'/index.html">'${PKGNAME}' Documentation ...</a></html>' > target/docs/index.html

# Alias to run targets: fmtcheck test vet lint coverage
qa: fmtcheck test vet lint coverage

# --- INSTALL ---

# Get the dependencies
deps:
	GOPATH=$(GOPATH) go get ./...
	GOPATH=$(GOPATH) go get github.com/golang/lint/golint
	GOPATH=$(GOPATH) go get github.com/jstemmer/go-junit-report
	GOPATH=$(GOPATH) go get github.com/pebbe/zmq4
	GOPATH=$(GOPATH) go get github.com/codegangsta/martini
	GOPATH=$(GOPATH) go get github.com/bobappleyard/readline

# Install this application
install: uninstall
	mkdir -p $(PATHINSTBIN)
	cp -r ./target/bin/* $(PATHINSTBIN)
	find $(PATHINSTBIN) -type d -exec chmod 755 {} \;
	find $(PATHINSTBIN) -type f -exec chmod 755 {} \;
	mkdir -p $(PATHINSTCFG)
	touch -c $(PATHINSTCFG)*
	cp -ru ./resources/etc/workerpoolmanager/* $(PATHINSTCFG)
	chmod -R 644 $(PATHINSTCFG)*
	mkdir -p $(PATHINSTDOC)
	cp -f ./README.md $(PATHINSTDOC)
	cp -f ./VERSION $(PATHINSTDOC)
	cp -f ./RELEASE $(PATHINSTDOC)
	chmod -R 644 $(PATHINSTDOC)*

# Remove all installed files (excluding configuration files)
uninstall:
	rm -rf $(PATHINSTBIN)
	rm -rf $(PATHINSTDOC)

# Remove any build artifact
clean:
	GOPATH=$(GOPATH) go clean ./...

# Deletes any intermediate file
nuke:
	rm -rf ./target
	GOPATH=$(GOPATH) go clean -i ./...

# Compile the application
build: deps
	GOPATH=$(GOPATH) go build -o ./target/bin/wpmanager ./wpmanager
	GOPATH=$(GOPATH) go build -o ./target/bin/wpconsole ./wpconsole

# --- PACKAGING ---

# Build the RPM package for RedHat-like Linux distributions
rpm:
	rm -rf $(PATHRPMPKG)
	rpmbuild --define "_topdir $(PATHRPMPKG)" --define "_package $(PKGNAME)" --define "_version $(VERSION)" --define "_release $(RELEASE)" --define "_current_directory $(CURRENTDIR)" --define "_binpath /$(BINPATH)" --define "_docpath /$(DOCPATH)" --define "_configpath /$(CONFIGPATH)" -bb resources/rpm/rpm.spec

# Execute all tests and build the RPM package
dist: qa rpm
