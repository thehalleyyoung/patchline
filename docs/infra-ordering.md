# Infrastructure/data ordering analysis

A migration that runs in the wrong order relative to its deploy is a classic outage: the application
ships before the schema is ready, or a destructive cleanup runs before traffic has drained. Patchline
now performs a derived **ordering analysis** that correlates deploy-ordering markers with the
migration and database jobs detected on the same manifest, across the infrastructure systems real
teams use:

| Ordering marker | Source |
|-----------------|--------|
| `helm.sh/hook` (pre/post-install, pre/post-upgrade) | Helm |
| `argocd.argoproj.io/sync-wave` | Argo CD |
| `initContainers`, `depends-on` | Kubernetes |
| Terraform `depends_on`, `wait = true`, `atomic = true` | Terraform/Helm provider |

For every migration or database job, Patchline emits a searchable `infra_data_ordering` fact
classified as:

- **sequenced** (`ordered=true`) — the data-change job is gated by an explicit ordering marker, so it
  runs in a defined relationship to the rollout.
- **unordered** (`ordered=false`) — the data-change job has no ordering marker and can race the
  deploy.

Guarantees enforced by the gate:

1. Both classifications are proven against the real `helm/charts` repository, where well-known charts
   such as **Kong** and **Anchore** sequence their migration jobs with Helm pre/post-upgrade hooks
   while other database jobs run unordered.
2. The ordered/unordered classification is verified by a deterministic unit test covering an
   unordered Kubernetes migration Job and a Helm-hooked sequenced migration Job.

```
make infra-ordering-gate
```

Outputs land in `results/generated/infra-ordering/`.
