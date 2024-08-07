test-coverage:
	@go test -coverprofile=coverage.out ./pkg/... ./internal/...
	@go tool cover -html="coverage.out" -o coverage.html
	@echo "Total Coverage: `go tool cover -func=coverage.out | grep total | grep -Eo '[0-9]+\.[0-9]+'` %"

docs:
	@echo "Docs available at http://localhost:6060/github.com/luikyv/go-oidc"
	@pkgsite -http=:6060
