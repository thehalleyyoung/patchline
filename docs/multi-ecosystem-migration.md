# Multi-ecosystem migration coverage

Patchline started with Rails, Django, Alembic, Flyway, and Prisma. Real repositories use many more
migration frameworks, and each one hides its data-change surface in a different place. The inventory
now recognizes seven additional ecosystems from their on-disk conventions and maps each to the
**project-native migration command** a maintainer would actually run:

| Framework | Detected from | Native command |
|-----------|---------------|----------------|
| **Laravel** (PHP) | `database/migrations/*.php` | `php artisan migrate` |
| **Doctrine** (PHP) | `Migrations/VersionYYYY….php` | `php bin/console doctrine:migrations:migrate` |
| **Ecto** (Elixir) | `priv/repo/migrations/*.exs` | `mix ecto.migrate` |
| **Diesel** (Rust) | `diesel.toml`, `migrations/*/up.sql` | `diesel migration run` |
| **Sequelize** (JS) | `.sequelizerc`, `sequelize/migrations/` | `npx sequelize-cli db:migrate` |
| **Knex** (JS) | `knexfile.js`/`knexfile.ts` | `npx knex migrate:latest` |
| **Rails multi-db** | `db/<name>_migrate/` | `bundle exec rails db:migrate:<name>` |

Rails multi-database support extracts the per-database name from the path so the recommended command
targets the right database (e.g. `db/animals_migrate/` → `rails db:migrate:animals`).

Guarantees enforced by the gate:

1. **Laravel** detection is proven against the real `laravel/laravel` repository, including the
   `php artisan migrate` native command.
2. The full **seven-framework matrix** and each framework's native command are verified by
   deterministic unit tests.

```
make multi-ecosystem-migration-gate
```

Outputs land in `results/generated/multi-ecosystem-migration/`.
