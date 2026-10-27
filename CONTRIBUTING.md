# Contributing to agentprs

Thanks for taking the time to help. Bug reports, ideas, docs fixes and code are all welcome.

## Before you start

- **Bugs:** open an [issue](https://github.com/celaldrg/agentprs/issues/new/choose) with steps to reproduce.
- **Bigger changes:** open an issue first so we can agree on the approach before you spend time on it.
- **First contribution?** Look for issues labeled [`good first issue`](https://github.com/celaldrg/agentprs/labels/good%20first%20issue).

## Development setup

You need Go 1.24 or newer. There are no third-party dependencies.

```sh
git clone https://github.com/celaldrg/agentprs
cd agentprs
go test ./...
go run . vercel/next.js        # set GITHUB_TOKEN to avoid the 60 calls/hour limit
go run . --input testdata/sample.json   # offline, from a fixture
```

## Adding a new agent

Most contributions will be new agent signatures. They live in [`internal/detect/detect.go`](internal/detect/detect.go).

1. Find a **public, self-declared** signal: the GitHub App that opens the PR, or a footer or link the tool writes into the PR body by itself. Link to an example PR in your pull request.
2. Make the pattern strict enough that it can never match a human (branch names like `copilot/…` are used by people too, so they are not enough on their own).
3. Add a positive *and* a negative case to [`internal/detect/detect_test.go`](internal/detect/detect_test.go).

## Pull requests

1. Fork the repository and create a branch from `main`.
2. Keep the change focused: one fix or feature per pull request.
3. Add or update tests for any behavior change. CI must pass.
4. Update the README if you change flags, output or behavior users can see.
5. Describe what changed and why in the pull request.

Commit messages: short imperative summary (`Add --quiet flag`, `Fix parsing of quoted values`).

## Code of Conduct

This project follows the [Contributor Covenant](CODE_OF_CONDUCT.md). By taking part you agree to follow it.

## License

By contributing, you agree that your contributions are licensed under the [MIT License](LICENSE).
