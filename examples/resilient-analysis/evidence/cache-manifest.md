# Cache manifest

`invoice-delete-cache-artifact` was intentionally seeded with stale bytes. Patchline must hash the artifact, quarantine the stale entry, rebuild it, and accept only the rebuilt result hash.
