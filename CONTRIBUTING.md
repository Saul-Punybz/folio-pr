# Contributing to Folio

Thank you for your interest in contributing to Folio! This project monitors Puerto Rico news and government activity to promote transparency and informed civic engagement.

## How to Contribute

### Reporting Bugs
- Open a [GitHub Issue](https://github.com/Saul-Punybz/folio-pr/issues/new)
- Include steps to reproduce, expected behavior, and actual behavior
- Include your OS, Go version, and Node version

### Suggesting Features
- Open a GitHub Issue with the "enhancement" label
- Describe the use case and why it would be valuable

### Submitting Code

1. Fork the repository
2. Create a feature branch: `git checkout -b feature/my-feature`
3. Make your changes
4. Build and verify:
   ```bash
   go build ./cmd/api
   cd frontend && npm run build
   ```
5. Commit with a clear message
6. Push and open a Pull Request

### Code Style
- **Go**: Follow standard Go conventions (`gofmt`, `go vet`)
- **TypeScript/React**: Use the existing patterns in `frontend/src/islands/`
- **SQL**: Migrations are numbered sequentially (`010_*.sql`, `011_*.sql`, etc.)

### Adding News Sources
To add a new Puerto Rico news source:
1. Add the source configuration in `migrations/` as a new migration
2. Determine if it's RSS or HTML scrape
3. For HTML scrape sources, identify the CSS selectors for article links, titles, and body text
4. Test with `POST /api/admin/ingest`

## Project Structure

See the [README](README.md) for full architecture details.

## Code of Conduct

Be respectful, constructive, and collaborative. This project serves the public interest.

## Questions?

Open an issue or start a discussion on GitHub.

---

**Maintainer:** [Saul Gonzalez Alonso](https://github.com/Saul-Punybz)
