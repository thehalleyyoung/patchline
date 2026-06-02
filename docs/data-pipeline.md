# Data-pipeline repair evidence

Relational and NoSQL stores are not the only places data changes destructively. Modern stacks move
and reshape data through orchestration and compute frameworks, where a single overwrite can erase a
table just as surely as `DROP TABLE`. Patchline's inventory now records data-pipeline repair
evidence across four frameworks, scanning each file only when a framework-specific **signal**
confirms the target so prose and unrelated code are not misclassified:

| Framework | Signals | Destructive operations flagged |
|-----------|---------|--------------------------------|
| **Airflow** | `from airflow`, `DAG(`, `@dag`, `airflow.operators` | embedded `DROP`/`TRUNCATE` SQL, `backfill`, `clear_task_instances`, dagrun reset |
| **dbt** | `{{ config(`, `{{ ref(`, `dbt_project.yml` | `--full-refresh`, `full_refresh=true`, `materialized='table'` |
| **Spark** | `pyspark`, `SparkSession`, `org.apache.spark` | `.mode("overwrite")`, `saveAsTable`, `insertInto` |
| **Kafka** | `KafkaConsumer`, `@KafkaListener`, `bootstrap.servers` | `auto.offset.reset`, `seekToBeginning`, `--reset-offsets` |

Every matched operation becomes a `data_pipeline_change` fact carrying `framework`, `operation`, and
a `destructive` flag, so destructive pipeline changes surface as searchable evidence alongside SQL
and NoSQL migration risks.

Guarantees enforced by the gate:

1. Multi-framework destructive detection is proven against the real
   `harrydevforlife/building-lakehouse` repository, which combines Airflow orchestration, dbt
   transforms, and Spark overwrite writes.
2. The full **four-framework matrix** (Airflow, dbt, Spark, Kafka) and a **no-false-positive** rule
   for prose are verified by deterministic unit tests.

```
make data-pipeline-gate
```

Outputs land in `results/generated/data-pipeline/`.
