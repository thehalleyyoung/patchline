# Source SQL extraction

Application code often hides data effects outside migration files. Patchline's `extract-sql` command builds a deterministic inventory of embedded SQL, migration-framework DSL calls, and ORM/query-builder operations:

```bash
go run ./cmd/patchline extract-sql examples/source-sql
go run ./cmd/patchline extract-sql examples/source-sql --json
```

The extractor scans Go, Python, TypeScript/JavaScript, Ruby, Java, C#, shell scripts, `.sql` migration files, and common migration-framework layouts such as Rails `db/migrate`, Alembic, Prisma migrations, Flyway-style SQL, and Liquibase paths.

It emits two conservative evidence kinds:

| Kind | Meaning |
| --- | --- |
| `embedded_sql` | A SQL-looking literal or heredoc with a normalized fingerprint, table, operation, risk, and effect from the migration analyzer when supported. |
| `orm_query` / `migration_framework` | A framework/query-builder observation with framework, operation, table, confidence, and snippet hash. |

Supported framework detectors include Rails ActiveRecord/migrations, Django querysets, Prisma Client/migrations, TypeORM, Sequelize, Entity Framework, Knex, Alembic, and generic query builders. The detector does not pretend ORM calls are full SQL: it records them as lower-confidence source evidence unless a raw SQL literal is present.

The fixture corpus under `examples/source-sql` exercises all supported language families and frameworks. It gives the semantic audit an immediate historical-code input, so Patchline can connect migration and repair semantics to SQL effects that would otherwise be invisible in application source.
