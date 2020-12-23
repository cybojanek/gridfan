
.phony: coverage help test

define HELP_BODY
clean
format
gridfan
help
endef

help:
	$(info $(HELP_BODY))

################################################################################

clean:
	rm -f gridfan

format:
	gofmt -s -w cmd internal

gridfan: cmd/gridfan/gridfan.go $(shell find internal -name '*.go')
	go build -v ./cmd/gridfan

