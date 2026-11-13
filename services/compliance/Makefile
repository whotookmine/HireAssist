# Short commands for the Compliance & Insights Service. No Go toolchain is assumed: each target
# runs the compiler in a container, as the Hiring Service's README describes.

GO := docker run --rm -v "$(CURDIR)":/src -w /src -v hireassist-gomod:/go/pkg/mod golang:1.25-alpine

# The local stack from the root docker-compose.yml. Development only; never real credentials.
# The test container joins the stack's network, so it reaches the database by its service name.
INTEGRATION := COMPLIANCE_TEST_DATABASE_URL='postgres://compliance:compliance@compliance-db:5432/compliance_test?sslmode=disable'

.PHONY: help test test-all

help:
	@echo "make test      unit and adapter tests"
	@echo "make test-all  the same plus the PostgreSQL integration test (needs: docker compose up -d compliance-db)"

test:
	$(GO) go test ./...

test-all:
	docker run --rm --network hireassist_default -v "$(CURDIR)":/src -w /src -v hireassist-gomod:/go/pkg/mod \
		golang:1.25-alpine sh -c "$(INTEGRATION) go test -count=1 ./..."
