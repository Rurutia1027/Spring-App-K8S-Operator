# CI

GitHub Actions: `.github/workflows/ci.yml`

| Job | Checks |
|-----|--------|
| `app-test` | Maven unit tests (H2, CRUD API) |
| `go-check` | golangci-lint, go vet, envtest unit tests |
| `kind-e2e` | Kind cluster: build/load images, deploy operator, postgres + app smoke + SpringApp CR |

Local kind smoke: `sh scripts/ci-kind-smoke.sh` (requires kind cluster `spring-app`).
