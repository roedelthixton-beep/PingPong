# Infrastructure TODO List

Continuous improvement items, ordered by priority.

## High Priority

### CI/CD Configuration
- [ ] GitHub Actions workflow: automated testing, linting (golint, go vet), build verification, coverage reports

### Test Coverage
- [ ] Run coverage analysis (`go test -coverprofile=coverage.out ./...`)
- [ ] Add unit tests for core functionality
- [ ] Target: at least 60% code coverage
- [ ] Integrate coverage checks in CI

### API Documentation
- [ ] Ensure all API endpoints have complete Swagger annotations
- [ ] Add request/response examples and error code documentation
- [ ] Create Postman Collection

## Medium Priority

### Docker Optimization
- [ ] Add Dockerfile (multi-stage build)
- [ ] Publish pre-built Docker images (Docker Hub or GHCR)
- [ ] Add Kubernetes deployment examples

### Documentation Structure
- [ ] Add bilingual (EN/ZH) documentation
- [ ] Create documentation index page

### Examples and Tutorials
- [ ] Create `examples/` directory
- [ ] Add usage examples (basic, advanced, integration)
- [ ] Write a complete getting-started tutorial

### Feed and Item Pipeline Documentation
- [ ] `feed_service_design.md`: add latest caching strategy (SearchCache + ProfileCache + SingleFlight)
- [ ] `item_pipeline_design.md`: add Elasticsearch index structure and ILM policy
- [ ] `feedback_milestone_flow_design.md`: add notification sorting implementation details

## Low Priority

### CHANGELOG.md
- [ ] Create CHANGELOG.md following Keep a Changelog format + Semantic Versioning

### SECURITY.md
- [ ] Create security policy with vulnerability reporting process and contact info

### Project Badges
- [ ] Add Build Status, Coverage, Go Report Card, License badges to README

### Performance Benchmarks
- [ ] Add benchmarks (`go test -bench=. -benchmem`)
- [ ] Document performance metrics

### Dependency Management
- [ ] Audit `go.mod`, remove unused dependencies
- [ ] Check dependency license compatibility
- [ ] `go mod tidy`

### Code Quality
- [ ] Run `golangci-lint run`, fix warnings
- [ ] Add comments for exported functions and types
- [ ] Remove dead code

### Project Website
- [ ] GitHub Pages + MkDocs/Docusaurus

### Internationalization
- [ ] Unify code comments in English
- [ ] Multi-language error messages
