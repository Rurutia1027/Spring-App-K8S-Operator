# Local Deploy (Kind)

Run from `operator/` after `kind-up` + `init-operator`.

| Command | What it does |
|---------|--------------|
| `make docker-build IMG=spring-app-operator:dev` | Build operator controller image |
| `make docker-build-app APP_IMG=notes-service:dev` | Build Spring Boot app image |
| `kind load docker-image ... --name spring-app` | Load local images into Kind |
| `make install` | Install SpringApp CRD into the cluster |
| `make deploy IMG=spring-app-operator:dev` | Deploy operator (Deployment + RBAC) |
| `make demo-up` | Apply demo CRs (Postgres + SpringApp); operator creates the app |

`install` = CRD only. `deploy` = operator only. `demo-up` does not start the app directly—the operator reconciles the SpringApp CR.
