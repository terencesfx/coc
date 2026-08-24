GO_ENV := GOEXPERIMENT=none GOCACHE=$(CURDIR)/.data/go-build GOMODCACHE=$(CURDIR)/.data/go-mod

.PHONY: dev-api dev-web create-admin restore test fmt check

dev-api:
	$(GO_ENV) go run ./cmd/server

dev-web:
	npm --prefix web run dev

create-admin:
	$(GO_ENV) go run ./cmd/admin create

restore:
	$(GO_ENV) go run ./cmd/admin restore -bundle "$(BUNDLE)" -confirm RESTORE

test:
	$(GO_ENV) go test ./cmd/... ./internal/...
	npm --prefix web run test

fmt:
	gofmt -w $$(rg --files cmd internal -g '*.go')
	npm --prefix web run format

check:
	$(GO_ENV) go vet ./cmd/... ./internal/...
	npm --prefix web run typecheck
	npm --prefix web run lint
