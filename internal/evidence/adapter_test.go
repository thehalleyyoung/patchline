package evidence

import (
	"strings"
	"testing"
)

func TestAdaptOTLPSpansToEvidence(t *testing.T) {
	input := `{
	  "resourceSpans": [{
	    "resource": {"attributes": [
	      {"key":"service.name","value":{"stringValue":"billing-api"}},
	      {"key":"git.commit.sha","value":{"stringValue":"8f3c2ab"}},
	      {"key":"patchline.deploy_id","value":{"stringValue":"2026-05-29T12:00Z"}}
	    ]},
	    "scopeSpans": [{
	      "spans": [{
	        "traceId":"abc",
	        "spanId":"def",
	        "name":"UPDATE invoices SET total_cents = 0",
	        "attributes": [
	          {"key":"patchline.migration_id","value":{"stringValue":"migration:bad_backfill"}},
	          {"key":"db.statement","value":{"stringValue":"UPDATE invoices SET total_cents = 0 WHERE status = 'open'"}},
	          {"key":"patchline.record_id","value":{"stringValue":"invoices/inv_1002"}}
	        ]
	      }]
	    }]
	  }]
	}`

	result, err := AdaptJSON(strings.NewReader(input), "otlp")
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK || result.EventCount != 5 {
		t.Fatalf("unexpected adapter result: %#v", result)
	}
	if result.Events[2]["type"] != "row_mutation" || result.Events[2]["record"] != "record:invoices/inv_1002" {
		t.Fatalf("expected row mutation event first after sorting: %#v", result.Events)
	}
	if result.Events[4]["type"] != "trace" || result.Events[4]["migration"] != "migration:bad_backfill" {
		t.Fatalf("expected trace event: %#v", result.Events)
	}
	ingested, err := IngestJSONL(strings.NewReader(eventsJSONL(result.Events)))
	if err != nil {
		t.Fatal(err)
	}
	if !ingested.OK {
		t.Fatalf("adapted evidence should ingest cleanly: %#v", ingested.Errors)
	}
}

func TestAdaptOTLPCollectorLogsToEvidence(t *testing.T) {
	input := `{
	  "resourceLogs": [{
	    "resource": {"attributes": [
	      {"key":"service.name","value":{"stringValue":"billing-api"}},
	      {"key":"git.commit.sha","value":{"stringValue":"8f3c2ab"}},
	      {"key":"patchline.deploy_id","value":{"stringValue":"2026-05-29T12:00Z"}}
	    ]},
	    "scopeLogs": [{
	      "logRecords": [{
	        "traceId": "abc",
	        "spanId": "def",
	        "severityText": "ERROR",
	        "body": {"stringValue": "UPDATE invoices SET total_cents = 0 WHERE status = 'open' failed"},
	        "attributes": [
	          {"key":"patchline.migration_id","value":{"stringValue":"migration:bad_backfill"}},
	          {"key":"db.statement","value":{"stringValue":"UPDATE invoices SET total_cents = 0 WHERE status = 'open'"}}
	        ]
	      }]
	    }]
	  }]
	}`

	result, err := AdaptJSON(strings.NewReader(input), "otlp")
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	for _, event := range result.Events {
		counts[event["type"]]++
	}
	for _, eventType := range []string{"deploy", "migration", "trace", "sql_mutation", "log"} {
		if counts[eventType] == 0 {
			t.Fatalf("expected %s event in %#v", eventType, result.Events)
		}
	}
	ingested, err := IngestJSONL(strings.NewReader(eventsJSONL(result.Events)))
	if err != nil {
		t.Fatal(err)
	}
	if !ingested.OK {
		t.Fatalf("adapted OTLP log evidence should ingest cleanly: %#v events=%#v", ingested.Errors, result.Events)
	}
}

func TestAdaptDatadogSpanAndDeployEvent(t *testing.T) {
	input := `{
	  "events": [{
	    "id": "20260529",
	    "title": "deploy",
	    "tags": ["service:billing-api", "git.commit.sha:8f3c2ab", "patchline.deploy_id:2026-05-29T12:00Z"]
	  }],
	  "spans": [{
	    "trace_id": "123",
	    "span_id": "456",
	    "resource": "UPDATE invoices SET total_cents = ?",
	    "meta": {
	      "patchline.migration_id": "bad_backfill",
	      "patchline.deploy_id": "2026-05-29T12:00Z",
	      "db.statement": "UPDATE invoices SET total_cents = ? WHERE status = ?"
	    }
	  }]
	}`

	result, err := AdaptJSON(strings.NewReader(input), "datadog")
	if err != nil {
		t.Fatal(err)
	}
	if result.EventCount != 4 {
		t.Fatalf("unexpected event count: %#v", result)
	}
	var sawDeploy bool
	for _, event := range result.Events {
		if event["type"] == "deploy" {
			sawDeploy = true
			if event["id"] != "deploy:2026-05-29T12:00Z" || event["commit"] != "commit:8f3c2ab" {
				t.Fatalf("unexpected deploy event: %#v", event)
			}
		}
	}
	if !sawDeploy {
		t.Fatalf("expected deploy event: %#v", result.Events)
	}
}

func TestAdaptDatadogOperationalExports(t *testing.T) {
	input := `{
	  "events": [{
	    "id": "dep-1",
	    "title": "Deployment completed",
	    "tags": ["service:billing-api", "git.commit.sha:8f3c2ab", "patchline.deploy_id:2026-05-29T12:00Z"]
	  }],
	  "incidents": [{
	    "public_id": "INC-42",
	    "title": "Billing rows corrupted",
	    "state": "resolved",
	    "root_cause": "bad migration",
	    "tags": ["service:billing-api", "deployment.id:2026-05-29T12:00Z"]
	  }],
	  "logs": [{
	    "id": "log-1",
	    "message": "rollback started for table invoices",
	    "status": "error",
	    "ddsource": "postgres",
	    "service": "billing-api",
	    "trace_id": "abc"
	  }],
	  "monitors": [{
	    "id": "mon-1",
	    "name": "Billing DB error rate",
	    "type": "query alert",
	    "query": "sum(last_5m):sum:postgres.errors{service:billing-api} > 0",
	    "message": "billing database errors",
	    "tags": ["service:billing-api"]
	  }],
	  "slos": [{
	    "id": "slo-1",
	    "name": "Billing availability",
	    "target_threshold": 99.9,
	    "timeframe": "30d",
	    "tags": ["service:billing-api"]
	  }],
	  "notebooks": [{
	    "id": "nb-1",
	    "name": "Billing incident investigation",
	    "cells": [{"type": "markdown", "content": "incident INC-42 deploy 2026-05-29T12:00Z"}],
	    "tags": ["service:billing-api"]
	  }],
	  "spans": [{
	    "trace_id": "abc",
	    "span_id": "def",
	    "resource": "UPDATE invoices SET total_cents = ?",
	    "meta": {
	      "patchline.migration_id": "bad_backfill",
	      "patchline.deploy_id": "2026-05-29T12:00Z",
	      "git.commit.sha": "8f3c2ab",
	      "service": "billing-api",
	      "db.statement": "UPDATE invoices SET total_cents = ? WHERE status = ?"
	    }
	  }]
	}`

	result, err := AdaptJSON(strings.NewReader(input), "datadog")
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	for _, event := range result.Events {
		counts[event["type"]]++
	}
	for _, eventType := range []string{"deploy", "incident", "log", "monitor", "slo", "notebook", "trace", "sql_mutation"} {
		if counts[eventType] == 0 {
			t.Fatalf("expected %s event in %#v", eventType, result.Events)
		}
	}
	ingested, err := IngestJSONL(strings.NewReader(eventsJSONL(result.Events)))
	if err != nil {
		t.Fatal(err)
	}
	if !ingested.OK {
		t.Fatalf("adapted datadog operational evidence should ingest cleanly: %#v events=%#v", ingested.Errors, result.Events)
	}
}

func TestAdaptWarnsWhenSQLSpanLacksMigration(t *testing.T) {
	input := `{"resourceSpans":[{"scopeSpans":[{"spans":[{"traceId":"abc","spanId":"def","attributes":[{"key":"db.statement","value":{"stringValue":"select 1"}}]}]}]}]}`
	result, err := AdaptJSON(strings.NewReader(input), "otlp")
	if err != nil {
		t.Fatal(err)
	}
	if result.EventCount != 0 || len(result.Warnings) != 1 {
		t.Fatalf("expected skipped SQL span warning: %#v", result)
	}
}

func TestAdaptPostgresLogicalDecoding(t *testing.T) {
	input := `{
	  "patchline_migration_id": "20260529_bad_invoice_backfill",
	  "patchline_deploy_id": "2026-05-29T12:00Z",
	  "commit": "8f3c2ab",
	  "service": "billing-api",
	  "xid": 991,
	  "change": [{
	    "kind": "update",
	    "schema": "public",
	    "table": "invoices",
	    "columnnames": ["id", "total_cents"],
	    "columnvalues": ["inv_1002", "0"],
	    "oldkeys": {"keynames": ["id"], "keyvalues": ["inv_1002"]}
	  }]
	}`
	result, err := AdaptJSON(strings.NewReader(input), "postgres")
	if err != nil {
		t.Fatal(err)
	}
	if result.EventCount != 5 {
		t.Fatalf("unexpected result: %#v", result)
	}
	ingested, err := IngestJSONL(strings.NewReader(eventsJSONL(result.Events)))
	if err != nil {
		t.Fatal(err)
	}
	if !ingested.OK {
		t.Fatalf("adapted postgres evidence should ingest cleanly: %#v", ingested.Errors)
	}
}

func TestAdaptGitHubDeploymentsAndReleases(t *testing.T) {
	input := `{
	  "repository": {"name": "billing-api"},
	  "deployments": [{"id": 42, "sha": "8f3c2ab", "environment": "production", "payload": {"service": "billing-api"}}],
	  "releases": [{"id": 77, "tag_name": "v1.2.3", "target_commitish": "main", "repository": {"name": "billing-api"}}]
	}`
	result, err := AdaptJSON(strings.NewReader(input), "github")
	if err != nil {
		t.Fatal(err)
	}
	if result.EventCount != 2 {
		t.Fatalf("unexpected github adapter result: %#v", result)
	}
	if result.Events[0]["type"] != "deploy" || result.Events[0]["commit"] == "" {
		t.Fatalf("expected deploy evidence: %#v", result.Events)
	}
}

func TestAdaptMigrationRunner(t *testing.T) {
	input := `{
	  "tool": "goose",
	  "deploy_id": "2026-05-29T12:00Z",
	  "commit": "8f3c2ab",
	  "service": "billing-api",
	  "migrations": [{
	    "version": "20260529_bad_invoice_backfill",
	    "name": "bad invoice total backfill",
	    "status": "applied",
	    "trace_id": "runner-991",
	    "sql_fingerprint": "UPDATE invoices SET total_cents = ? WHERE status = ?"
	  }]
	}`
	result, err := AdaptJSON(strings.NewReader(input), "migration-runner")
	if err != nil {
		t.Fatal(err)
	}
	if result.EventCount != 4 {
		t.Fatalf("unexpected runner adapter result: %#v", result)
	}
	ingested, err := IngestJSONL(strings.NewReader(eventsJSONL(result.Events)))
	if err != nil {
		t.Fatal(err)
	}
	if !ingested.OK {
		t.Fatalf("adapted runner evidence should ingest cleanly: %#v", ingested.Errors)
	}
}

func eventsJSONL(events []map[string]string) string {
	var lines []string
	for _, event := range events {
		parts := []string{}
		for _, key := range []string{"type", "id", "commit", "service", "deploy", "name", "migration", "trace", "fingerprint", "record", "sql", "title", "status", "message", "query"} {
			if value := event[key]; value != "" {
				parts = append(parts, `"`+key+`":"`+value+`"`)
			}
		}
		lines = append(lines, "{"+strings.Join(parts, ",")+"}")
	}
	return strings.Join(lines, "\n")
}
