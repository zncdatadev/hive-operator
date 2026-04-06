<!-- Parent: ../AGENTS.md -->
<!-- Generated: 2026-04-06 | Updated: 2026-04-06 -->

# hive-operator

## Purpose
Manages Apache Hive deployments on Kubernetes. Handles creation, configuration, and lifecycle management of Hive metastore instances including schema initialization, S3/HDFS storage integration, and database backend configuration.

## Key Files
| File | Description |
|------|-------------|
| `go.mod` | Go module dependencies (module: `github.com/zncdatadev/hive-operator`) |
| `Makefile` | Build, test, and deployment commands |
| `PROJECT` | Kubebuilder project metadata |
| `Dockerfile` | Operator container image build |

## Subdirectories
| Directory | Purpose |
|-----------|---------|
| `api/v1alpha1/` | CRD type definitions for HiveMetastore |
| `cmd/` | Operator entry point (`main.go`) |
| `config/` | Kustomize-based Kubernetes manifests (CRDs, RBAC, manager) |
| `deploy/` | Deployment manifests and Helm chart stubs |
| `examples/` | Example HiveMetastore CR manifests |
| `internal/` | Controller and reconciliation logic |
| `internal/controller/` | Reconciler implementations |
| `test/` | E2E test suites (Ginkgo/Gomega) |
| `hack/` | Development helper scripts |

## For AI Agents

### Working In This Directory
- Standard Kubebuilder operator structure with `operator-go` GenericReconciler framework
- Go module: `github.com/zncdatadev/hive-operator`, Go 1.25+
- Run `make test` for unit tests
- Run `make generate && make manifests` after modifying API types
- Run `make deploy` to deploy to a cluster (requires kubeconfig)
- Worktrees are stored under `../.hive-operator-worktrees/`

### Testing Requirements
- Unit tests: `make test` (uses envtest)
- E2E tests in `test/e2e/` — requires a live Kubernetes cluster
- Test framework: Ginkgo v2 + Gomega

### Common Patterns
- Controllers in `internal/controller/`
- CRDs use `v1alpha1` API version under `api/v1alpha1/`
- Follows `operator-go` GenericReconciler pattern
- Config generation uses `operator-go` config builder helpers

## Dependencies

### Internal
- `../operator-go` — Shared operator framework (`github.com/zncdatadev/operator-go v0.12.6`)

### External
- `sigs.k8s.io/controller-runtime v0.23+`
- `k8s.io/api`, `k8s.io/apimachinery`, `k8s.io/client-go v0.35+`
- Kubernetes 1.26+

<!-- MANUAL: Any manually added notes below this line are preserved on regeneration -->
