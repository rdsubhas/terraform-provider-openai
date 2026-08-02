# Contributing

This repository is independently maintained and does not synchronize changes from its
upstream fork source.

1. Open an issue describing the change or bug.
2. Create a focused branch and include tests for behavior changes.
3. Run `make build`, `make test`, `make lint`, and `go generate ./...`.
4. Commit generated documentation when provider schemas or examples change.
5. Open a pull request describing the impact and validation performed.

Acceptance tests require valid OpenAI credentials and may create billable resources.
Never commit credentials, Terraform state, or local environment files.
