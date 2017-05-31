.PHONY: all build stats test vendor

all: build

build:
	go install .

stats:
	cloc . --exclude-dir=vendor

test: check
	env FRONTEND=none govendor test +local

vendor:
	go get -u github.com/kardianos/govendor
	govendor fetch +outside
	govendor remove +unused
