# Contributing

Thanks for taking an interest in RouteBox Tracking Events.

## Workflow

1. Create a feature branch from `main`.
2. Make your changes, including tests where applicable.
3. Open a pull request describing what changed and why.
4. Ensure the Jenkins pipeline passes before requesting review.

## Code Standards

- Follow standard Go formatting (`gofmt`) before committing.
- Keep carrier-specific logic isolated in `internal/carriers`; avoid leaking carrier quirks into shared code paths.
- Prefer small, focused pull requests over large ones.

## Tests

- Add or update tests for any change to signature validation, deduplication, or event parsing logic.
- Run the test suite locally before opening a pull request.

## Reporting Issues

If you find a bug or a gap in documentation, please open an issue describing the behavior you observed and what you expected instead. For anything security-related, please avoid filing a public issue and reach out directly instead.
