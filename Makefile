##### Arguments ######
COLOR := "\e[1;36m%s\e[0m\n"

tag:
	git tag -a v$(VERSION) -m "Release version $(VERSION)"
	git push origin v$(VERSION)

lint:
	@printf $(COLOR) "Running linter"
	go mod tidy
	golangci-lint run
	govulncheck ./...

## Run test with coverage (for specific package)
test-math:
	@printf $(COLOR) "Running unit testing..."
	mkdir -p coverage
	go test -coverprofile=./coverage/coverage.out ./math && go tool cover -html=./coverage/coverage.out -o ./coverage/coverage.html