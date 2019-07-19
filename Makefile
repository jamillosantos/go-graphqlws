VERSION ?= $(shell git describe --tags 2>/dev/null | cut -c 2-)
TEST_FLAGS ?=
REPO_OWNER ?= $(shell cd .. && basename "$$(pwd)")

COVERDIR=$(CURDIR)/.cover
COVERAGEFILE=$(COVERDIR)/cover.out

EXAMPLES = $(shell ls examples)

.PHONY: test test-watch coverage coverage-ci coverage-html dep-ensure dep-update vet lint fmt benchmark-load install-examples example-echo-server

test:
	@ginkgo --failFast ./...

test-watch:
	@ginkgo watch -cover -r ./...

coverage:
	@mkdir -p $(COVERDIR)
	@ginkgo -r -covermode=count --cover --trace ./
	@echo "mode: count" > "${COVERAGEFILE}"
	@find . -type f -name *.coverprofile -exec grep -h -v "^mode:" {} >> "${COVERAGEFILE}" \; -exec rm -f {} \;

coverage-ci:
	@mkdir -p $(COVERDIR)
	@${GOPATHCMD} ginkgo -r -covermode=count --cover --trace ./
	@echo "mode: count" > "${COVERAGEFILE}"
	@find . -type f -name *.coverprofile -exec grep -h -v "^mode:" {} >> "${COVERAGEFILE}" \; -exec rm -f {} \;

coverage-html:
	@go tool cover -html="${COVERAGEFILE}" -o .cover/report.html

vet:
	@go vet ./...

lint:
	@golint

fmt:
	@go fmt ./...

benchmark-load:
ifndef SCENARIO
	@echo "Please define SCENARIO"
	@echo "make benchmark SCENARIO=<scenarios from './artillary' folder>"
else
	artillery run ./artillery/$(SCENARIO)
endif

install-examples:
ifdef PROFILER
	$(eval EXTRAARGS=)
endif
	@$(foreach example,$(EXAMPLES),GOBIN=$(CURDIR)/bin go go install "-ldflags=$(LDFLAGS)" $(EXTRAARGS) -v ./examples/$(example) &&) :

example-echo-server: install-examples
	RLOG_LOG_LEVEL=DEBUG ./bin/echo-server

example-simple-chat-server-redis: install-examples
	RLOG_LOG_LEVEL=DEBUG ./bin/simple-chat-server-redis