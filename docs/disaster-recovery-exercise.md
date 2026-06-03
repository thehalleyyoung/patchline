# Disaster recovery exercise

Patchline runs a **disaster recovery exercise** that proves the public corpus, docs, releases, and certificate logs can be **restored from mirrors** rather than merely declared durable.

The drill rebuilds the existing corpus release, docs site, artifact release manifest, and signed certificate revocation log. It publishes each component to primary and secondary mirrors with full-file SHA-256 manifests, corrupts or deletes primary copies for three components, restores from the first checksum-valid mirror, replays the restored certificate log, and rejects a negative control where every mirror has lost the required certificate bundle.

```bash
make disaster-recovery-exercise-gate
```
