package evidence

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/thehalleyyoung/patchline/internal/canonical"
)

const AdapterVersion = "patchline.evidence-adapter/v1"

type AdaptResult struct {
	Version    string              `json:"version"`
	Adapter    string              `json:"adapter"`
	OK         bool                `json:"ok"`
	EventCount int                 `json:"event_count"`
	InputHash  string              `json:"input_hash"`
	Events     []map[string]string `json:"events"`
	Warnings   []string            `json:"warnings,omitempty"`
}

func AdaptJSON(reader io.Reader, adapter string) (AdaptResult, error) {
	content, err := io.ReadAll(reader)
	if err != nil {
		return AdaptResult{}, err
	}
	normalized := strings.ToLower(strings.TrimSpace(adapter))
	result := AdaptResult{
		Version:   AdapterVersion,
		Adapter:   normalized,
		OK:        true,
		InputHash: canonical.HashBytes(bytes.TrimSpace(content)),
	}
	switch normalized {
	case "otlp":
		result.Events, result.Warnings, err = adaptOTLP(content)
	case "datadog":
		result.Events, result.Warnings, err = adaptDatadog(content)
	case "postgres", "postgres-logical", "wal2json":
		result.Events, result.Warnings, err = adaptPostgresLogical(content)
	case "github", "github-deployments":
		result.Events, result.Warnings, err = adaptGitHubDeployments(content)
	case "migration-runner", "runner":
		result.Events, result.Warnings, err = adaptMigrationRunner(content)
	default:
		err = fmt.Errorf("unknown evidence adapter %q", adapter)
	}
	if err != nil {
		return AdaptResult{}, err
	}
	result.Events = dedupeEvents(result.Events)
	sortEvents(result.Events)
	result.EventCount = len(result.Events)
	return result, nil
}

type postgresLogicalPayload struct {
	MigrationID string                 `json:"patchline_migration_id"`
	DeployID    string                 `json:"patchline_deploy_id"`
	Commit      string                 `json:"commit"`
	Service     string                 `json:"service"`
	XID         any                    `json:"xid"`
	LSN         string                 `json:"lsn"`
	Changes     []postgresLogicalEntry `json:"change"`
}

type postgresLogicalEntry struct {
	Kind         string             `json:"kind"`
	Schema       string             `json:"schema"`
	Table        string             `json:"table"`
	XID          any                `json:"xid"`
	LSN          string             `json:"lsn"`
	ColumnNames  []string           `json:"columnnames"`
	ColumnValues []any              `json:"columnvalues"`
	OldKeys      postgresLogicalKey `json:"oldkeys"`
}

type postgresLogicalKey struct {
	KeyNames  []string `json:"keynames"`
	KeyValues []any    `json:"keyvalues"`
}

func adaptPostgresLogical(content []byte) ([]map[string]string, []string, error) {
	var payload postgresLogicalPayload
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.UseNumber()
	if err := decoder.Decode(&payload); err != nil {
		return nil, nil, err
	}
	var events []map[string]string
	var warnings []string
	migrationID := firstNonEmpty(payload.MigrationID)
	if migrationID == "" {
		return nil, []string{"postgres logical decoding export has no patchline_migration_id"}, nil
	}
	deployID := firstNonEmpty(payload.DeployID)
	if deployID != "" && payload.Commit != "" && payload.Service != "" {
		events = append(events, deployAndMigrationEvents("postgres", deployID, payload.Commit, payload.Service, migrationID, "postgres logical decoding")...)
	}
	for index, change := range payload.Changes {
		if change.Kind == "" || change.Schema == "" || change.Table == "" {
			warnings = append(warnings, fmt.Sprintf("postgres change %d missing kind/schema/table", index))
			continue
		}
		if !isMutationKind(change.Kind) {
			continue
		}
		recordID := postgresRecordID(change)
		if recordID == "" {
			warnings = append(warnings, fmt.Sprintf("postgres change %d has no key value", index))
			continue
		}
		txID := firstNonEmpty(stringValue(change.XID), change.LSN, stringValue(payload.XID), payload.LSN, fmt.Sprintf("%d", index))
		traceEntity := "trace:postgres/" + cleanID(txID)
		sqlEntity := "sql:postgres/" + cleanID(txID) + "/" + cleanID(change.Schema+"."+change.Table+"/"+change.Kind)
		events = append(events,
			map[string]string{"type": "trace", "id": traceEntity, "migration": ensurePrefix(migrationID, "migration:"), "source": "postgres"},
			map[string]string{"type": "sql_mutation", "id": sqlEntity, "trace": traceEntity, "fingerprint": postgresFingerprint(change), "source": "postgres"},
			map[string]string{"type": "row_mutation", "record": recordID, "sql": sqlEntity, "source": "postgres"},
		)
	}
	return events, warnings, nil
}

func isMutationKind(kind string) bool {
	switch strings.ToLower(kind) {
	case "insert", "update", "delete":
		return true
	default:
		return false
	}
}

func postgresRecordID(change postgresLogicalEntry) string {
	schemaTable := change.Schema + "." + change.Table
	if len(change.OldKeys.KeyNames) > 0 && len(change.OldKeys.KeyValues) > 0 {
		return "record:" + schemaTable + "/" + cleanID(stringValue(change.OldKeys.KeyValues[0]))
	}
	for index, name := range change.ColumnNames {
		if (name == "id" || strings.HasSuffix(name, "_id")) && index < len(change.ColumnValues) {
			return "record:" + schemaTable + "/" + cleanID(stringValue(change.ColumnValues[index]))
		}
	}
	if len(change.ColumnValues) > 0 {
		return "record:" + schemaTable + "/" + cleanID(stringValue(change.ColumnValues[0]))
	}
	return ""
}

func postgresFingerprint(change postgresLogicalEntry) string {
	return strings.ToUpper(change.Kind) + " " + change.Schema + "." + change.Table
}

func adaptGitHubDeployments(content []byte) ([]map[string]string, []string, error) {
	var payload any
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.UseNumber()
	if err := decoder.Decode(&payload); err != nil {
		return nil, nil, err
	}
	var objects []map[string]any
	collectObjects(payload, &objects)
	var events []map[string]string
	var warnings []string
	for _, object := range objects {
		if event := githubDeployEvent(object); event != nil {
			events = append(events, event)
			continue
		}
		if event := githubReleaseEvent(object); event != nil {
			events = append(events, event)
		}
	}
	if len(events) == 0 {
		warnings = append(warnings, "github export did not contain deployment or release objects with commit information")
	}
	return events, warnings, nil
}

func githubDeployEvent(object map[string]any) map[string]string {
	if _, ok := object["environment"]; !ok {
		return nil
	}
	id := firstNonEmpty(stringValue(object["id"]), stringValue(object["node_id"]))
	commit := firstNonEmpty(stringValue(object["sha"]), stringValue(object["ref"]))
	service := firstNonEmpty(nestedString(object, "payload", "service"), nestedString(object, "repository", "name"), stringValue(object["environment"]))
	if id == "" || commit == "" || service == "" {
		return nil
	}
	return map[string]string{
		"type":    "deploy",
		"id":      "deploy:github/" + cleanID(id),
		"commit":  ensurePrefix(commit, "commit:"),
		"service": service,
		"source":  "github",
	}
}

func githubReleaseEvent(object map[string]any) map[string]string {
	if _, ok := object["tag_name"]; !ok {
		return nil
	}
	id := firstNonEmpty(stringValue(object["id"]), stringValue(object["tag_name"]))
	commit := firstNonEmpty(stringValue(object["target_commitish"]), nestedString(object, "target_commit", "sha"))
	service := firstNonEmpty(nestedString(object, "repository", "name"), stringValue(object["name"]))
	if id == "" || commit == "" || service == "" {
		return nil
	}
	return map[string]string{
		"type":    "deploy",
		"id":      "deploy:github-release/" + cleanID(id),
		"commit":  ensurePrefix(commit, "commit:"),
		"service": service,
		"source":  "github",
	}
}

type migrationRunnerPayload struct {
	Tool       string                   `json:"tool"`
	DeployID   string                   `json:"deploy_id"`
	Commit     string                   `json:"commit"`
	Service    string                   `json:"service"`
	Migrations []migrationRunnerEvent   `json:"migrations"`
	Events     []migrationRunnerEvent   `json:"events"`
	Runs       []migrationRunnerEvent   `json:"runs"`
	Metadata   map[string]string        `json:"metadata"`
	Raw        []map[string]interface{} `json:"-"`
}

type migrationRunnerEvent struct {
	ID             string `json:"id"`
	Version        string `json:"version"`
	Name           string `json:"name"`
	Status         string `json:"status"`
	DeployID       string `json:"deploy_id"`
	TraceID        string `json:"trace_id"`
	SQLFingerprint string `json:"sql_fingerprint"`
}

func adaptMigrationRunner(content []byte) ([]map[string]string, []string, error) {
	var payload migrationRunnerPayload
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.UseNumber()
	if err := decoder.Decode(&payload); err != nil {
		return nil, nil, err
	}
	runs := append([]migrationRunnerEvent{}, payload.Migrations...)
	runs = append(runs, payload.Events...)
	runs = append(runs, payload.Runs...)
	var events []map[string]string
	var warnings []string
	tool := firstNonEmpty(payload.Tool, payload.Metadata["tool"], "migration-runner")
	if payload.DeployID != "" && payload.Commit != "" && payload.Service != "" {
		events = append(events, map[string]string{
			"type":    "deploy",
			"id":      ensurePrefix(payload.DeployID, "deploy:"),
			"commit":  ensurePrefix(payload.Commit, "commit:"),
			"service": payload.Service,
			"source":  tool,
		})
	}
	for index, run := range runs {
		if run.Status != "" && !isSuccessfulMigrationStatus(run.Status) {
			continue
		}
		id := firstNonEmpty(run.ID, run.Version)
		deployID := firstNonEmpty(run.DeployID, payload.DeployID)
		if id == "" || deployID == "" {
			warnings = append(warnings, fmt.Sprintf("migration runner event %d missing id/version or deploy_id", index))
			continue
		}
		migrationID := ensurePrefix(id, "migration:")
		events = append(events, map[string]string{
			"type":   "migration",
			"id":     migrationID,
			"deploy": ensurePrefix(deployID, "deploy:"),
			"name":   firstNonEmpty(run.Name, id),
			"source": tool,
		})
		if run.TraceID != "" {
			traceID := ensurePrefix(run.TraceID, "trace:")
			events = append(events, map[string]string{"type": "trace", "id": traceID, "migration": migrationID, "source": tool})
			if run.SQLFingerprint != "" {
				events = append(events, map[string]string{
					"type":        "sql_mutation",
					"id":          "sql:" + cleanID(run.TraceID),
					"trace":       traceID,
					"fingerprint": run.SQLFingerprint,
					"source":      tool,
				})
			}
		}
	}
	return events, warnings, nil
}

func isSuccessfulMigrationStatus(status string) bool {
	switch strings.ToLower(status) {
	case "", "success", "succeeded", "applied", "complete", "completed", "up":
		return true
	default:
		return false
	}
}

func collectObjects(value any, objects *[]map[string]any) {
	switch typed := value.(type) {
	case []any:
		for _, item := range typed {
			collectObjects(item, objects)
		}
	case map[string]any:
		*objects = append(*objects, typed)
		for _, item := range typed {
			collectObjects(item, objects)
		}
	}
}

func nestedString(object map[string]any, keys ...string) string {
	var current any = object
	for _, key := range keys {
		next, ok := current.(map[string]any)
		if !ok {
			return ""
		}
		current = next[key]
	}
	return stringValue(current)
}

func deployAndMigrationEvents(source, deployID, commit, service, migrationID, name string) []map[string]string {
	return []map[string]string{
		{"type": "deploy", "id": ensurePrefix(deployID, "deploy:"), "commit": ensurePrefix(commit, "commit:"), "service": service, "source": source},
		{"type": "migration", "id": ensurePrefix(migrationID, "migration:"), "deploy": ensurePrefix(deployID, "deploy:"), "name": name, "source": source},
	}
}

type otlpPayload struct {
	ResourceSpans []otlpResourceSpan `json:"resourceSpans"`
	ResourceLogs  []otlpResourceLog  `json:"resourceLogs"`
}

type otlpResourceSpan struct {
	Resource                    otlpResource     `json:"resource"`
	ScopeSpans                  []otlpScopeSpans `json:"scopeSpans"`
	InstrumentationLibrarySpans []otlpScopeSpans `json:"instrumentationLibrarySpans"`
}

type otlpResource struct {
	Attributes []otlpAttribute `json:"attributes"`
}

type otlpScopeSpans struct {
	Spans []otlpSpan `json:"spans"`
}

type otlpResourceLog struct {
	Resource                  otlpResource    `json:"resource"`
	ScopeLogs                 []otlpScopeLogs `json:"scopeLogs"`
	InstrumentationLibraryLog []otlpScopeLogs `json:"instrumentationLibraryLogs"`
}

type otlpScopeLogs struct {
	LogRecords []otlpLogRecord `json:"logRecords"`
}

type otlpSpan struct {
	TraceID    string          `json:"traceId"`
	SpanID     string          `json:"spanId"`
	Name       string          `json:"name"`
	Attributes []otlpAttribute `json:"attributes"`
}

type otlpLogRecord struct {
	TraceID      string          `json:"traceId"`
	SpanID       string          `json:"spanId"`
	SeverityText string          `json:"severityText"`
	Body         otlpValue       `json:"body"`
	Attributes   []otlpAttribute `json:"attributes"`
}

type otlpAttribute struct {
	Key   string    `json:"key"`
	Value otlpValue `json:"value"`
}

type otlpValue struct {
	StringValue string  `json:"stringValue"`
	IntValue    string  `json:"intValue"`
	DoubleValue float64 `json:"doubleValue"`
	BoolValue   *bool   `json:"boolValue"`
}

func adaptOTLP(content []byte) ([]map[string]string, []string, error) {
	var payload otlpPayload
	if err := json.Unmarshal(content, &payload); err != nil {
		return nil, nil, err
	}
	var events []map[string]string
	var warnings []string
	for _, resourceSpan := range payload.ResourceSpans {
		resourceAttrs := attrsFromOTLP(resourceSpan.Resource.Attributes)
		scopes := append([]otlpScopeSpans{}, resourceSpan.ScopeSpans...)
		scopes = append(scopes, resourceSpan.InstrumentationLibrarySpans...)
		for _, scope := range scopes {
			for _, span := range scope.Spans {
				spanAttrs := mergeAttrs(resourceAttrs, attrsFromOTLP(span.Attributes))
				spanEvents, spanWarnings := adaptTraceSpan("otlp", span.TraceID, span.SpanID, span.Name, spanAttrs)
				events = append(events, spanEvents...)
				warnings = append(warnings, spanWarnings...)
			}
		}
	}
	for _, resourceLog := range payload.ResourceLogs {
		resourceAttrs := attrsFromOTLP(resourceLog.Resource.Attributes)
		scopes := append([]otlpScopeLogs{}, resourceLog.ScopeLogs...)
		scopes = append(scopes, resourceLog.InstrumentationLibraryLog...)
		for _, scope := range scopes {
			for _, record := range scope.LogRecords {
				logEvents, logWarnings := adaptOTLPLogRecord(resourceAttrs, record)
				events = append(events, logEvents...)
				warnings = append(warnings, logWarnings...)
			}
		}
	}
	return events, warnings, nil
}

func adaptOTLPLogRecord(resourceAttrs map[string]string, record otlpLogRecord) ([]map[string]string, []string) {
	attrs := mergeAttrs(resourceAttrs, attrsFromOTLP(record.Attributes))
	body := firstNonEmpty(valueFromOTLP(record.Body), attrs["body"], attrs["message"], attrs["log.message"])
	traceID := firstNonEmpty(record.TraceID, attrs["trace_id"], attrs["traceId"])
	spanID := firstNonEmpty(record.SpanID, attrs["span_id"], attrs["spanId"])
	traceEntity := ""
	if traceID != "" && spanID != "" {
		traceEntity = "trace:" + cleanID(traceID) + "/" + cleanID(spanID)
	} else if traceID != "" {
		traceEntity = ensurePrefix(cleanID(traceID), "trace:")
	}
	id := firstNonEmpty(attrs["log.record.uid"], attrs["log.iostream"], attrs["event.id"], traceEntity, body)
	if id == "" {
		return nil, []string{"otlp log record has no stable id, trace id, or body"}
	}
	event := map[string]string{
		"type":   "log",
		"id":     ensurePrefix(cleanID(id), "log:otlp/"),
		"source": "otlp",
	}
	for _, pair := range []struct {
		field  string
		values []string
	}{
		{"service", []string{attrs["service.name"], attrs["service"]}},
		{"trace", []string{traceEntity}},
		{"deploy", []string{attrs["patchline.deploy_id"], attrs["deployment.id"]}},
		{"migration", []string{attrs["patchline.migration_id"], attrs["db.migration.id"], attrs["migration.id"]}},
		{"commit", []string{attrs["git.commit.sha"], attrs["git.sha"], attrs["commit"]}},
		{"status", []string{record.SeverityText, attrs["severity"], attrs["log.level"]}},
		{"message", []string{body}},
	} {
		if found := firstNonEmpty(pair.values...); found != "" {
			event[pair.field] = normalizeDatadogEventField(pair.field, found)
		}
	}
	var events []map[string]string
	if traceID != "" && spanID != "" && firstNonEmpty(attrs["patchline.migration_id"], attrs["db.migration.id"], attrs["migration.id"]) != "" {
		spanEvents, spanWarnings := adaptTraceSpan("otlp", traceID, spanID, body, attrs)
		events = append(events, spanEvents...)
		events = append(events, event)
		return events, spanWarnings
	}
	return []map[string]string{event}, nil
}

func attrsFromOTLP(attributes []otlpAttribute) map[string]string {
	attrs := map[string]string{}
	for _, attr := range attributes {
		value := valueFromOTLP(attr.Value)
		if attr.Key != "" && value != "" {
			attrs[attr.Key] = value
		}
	}
	return attrs
}

func valueFromOTLP(value otlpValue) string {
	if value.StringValue != "" {
		return value.StringValue
	}
	if value.IntValue != "" {
		return value.IntValue
	}
	if value.DoubleValue != 0 {
		return fmt.Sprintf("%g", value.DoubleValue)
	}
	if value.BoolValue != nil {
		return fmt.Sprintf("%t", *value.BoolValue)
	}
	return ""
}

func adaptDatadog(content []byte) ([]map[string]string, []string, error) {
	var payload any
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.UseNumber()
	if err := decoder.Decode(&payload); err != nil {
		return nil, nil, err
	}
	var objects []map[string]any
	collectObjects(payload, &objects)

	var adapted []map[string]string
	var warnings []string
	for _, object := range objects {
		if event := datadogDeployEvent(object); event != nil {
			adapted = append(adapted, event)
		}
		if isDatadogSpan(object) {
			attrs := attrsFromDatadog(object)
			traceID := firstNonEmpty(stringValue(object["trace_id"]), stringValue(object["traceId"]), attrs["trace_id"])
			spanID := firstNonEmpty(stringValue(object["span_id"]), stringValue(object["spanId"]), attrs["span_id"])
			name := firstNonEmpty(stringValue(object["name"]), stringValue(object["resource"]))
			spanEvents, spanWarnings := adaptTraceSpan("datadog", traceID, spanID, name, attrs)
			adapted = append(adapted, spanEvents...)
			warnings = append(warnings, spanWarnings...)
		}
		if event := datadogOperationalEvent(object); event != nil {
			adapted = append(adapted, event)
		}
		if rawDashboard, ok := object["dashboard"]; ok {
			if nested := parseEmbeddedJSONObject(stringValue(rawDashboard)); nested != nil {
				if event := datadogOperationalEvent(nested); event != nil {
					adapted = append(adapted, event)
				}
			}
		}
	}
	if len(adapted) == 0 {
		warnings = append(warnings, "datadog export contained no deploy, incident, trace, log, monitor, SLO, or notebook objects")
	}
	return adapted, warnings, nil
}

func isDatadogSpan(value map[string]any) bool {
	_, hasTraceSnake := value["trace_id"]
	_, hasTraceCamel := value["traceId"]
	_, hasSpanSnake := value["span_id"]
	_, hasSpanCamel := value["spanId"]
	return (hasTraceSnake || hasTraceCamel) && (hasSpanSnake || hasSpanCamel)
}

func isDatadogEvent(value map[string]any) bool {
	_, hasTitle := value["title"]
	_, hasText := value["text"]
	_, hasTags := value["tags"]
	return hasTags && (hasTitle || hasText)
}

func datadogDeployEvent(value map[string]any) map[string]string {
	if !isDatadogEvent(value) {
		return nil
	}
	attrs := attrsFromDatadog(value)
	deployID := firstNonEmpty(attrs["patchline.deploy_id"], attrs["deployment.id"], stringValue(value["id"]))
	commit := firstNonEmpty(attrs["git.commit.sha"], attrs["git.sha"], attrs["commit"], attrs["revision"])
	service := firstNonEmpty(attrs["service"], attrs["service.name"])
	title := strings.ToLower(firstNonEmpty(stringValue(value["title"]), stringValue(value["text"]), attrs["event_type"], attrs["source"]))
	if !strings.Contains(title, "deploy") && !strings.Contains(title, "release") && attrs["deployment.id"] == "" && attrs["patchline.deploy_id"] == "" {
		return nil
	}
	if deployID == "" || commit == "" || service == "" {
		return nil
	}
	return map[string]string{
		"type":    "deploy",
		"id":      ensurePrefix(deployID, "deploy:"),
		"commit":  ensurePrefix(commit, "commit:"),
		"service": service,
		"source":  "datadog",
	}
}

func datadogOperationalEvent(value map[string]any) map[string]string {
	attrs := attrsFromDatadog(value)
	eventType := classifyDatadogOperationalKind(value, attrs)
	if eventType == "" {
		return nil
	}
	id := firstNonEmpty(stringValue(value["id"]), stringValue(value["public_id"]), attrs["id"], attrs[eventType+".id"], attrs["resource_name"], stringValue(value["name"]), stringValue(value["title"]))
	if id == "" {
		id = canonical.Hash(value)[:16]
	}
	event := map[string]string{
		"type":   eventType,
		"id":     ensurePrefix(cleanID(id), eventType+":datadog/"),
		"source": "datadog",
	}
	for _, pair := range []struct {
		field  string
		values []string
	}{
		{"service", []string{attrs["service"], attrs["service.name"], attrs["service_name"]}},
		{"deploy", []string{attrs["patchline.deploy_id"], attrs["deployment.id"], attrs["deploy_id"]}},
		{"migration", []string{attrs["patchline.migration_id"], attrs["db.migration.id"], attrs["migration.id"]}},
		{"trace", []string{attrs["trace_id"], attrs["traceId"], attrs["dd.trace_id"]}},
		{"commit", []string{attrs["git.commit.sha"], attrs["git.sha"], attrs["commit"], attrs["revision"]}},
		{"name", []string{stringValue(value["name"]), attrs["name"], stringValue(value["resource_name"])}},
		{"title", []string{stringValue(value["title"])}},
		{"status", []string{stringValue(value["status"]), stringValue(value["state"])}},
		{"message", []string{stringValue(value["message"]), stringValue(value["text"])}},
		{"query", []string{stringValue(value["query"]), attrs["query"]}},
	} {
		if found := firstNonEmpty(pair.values...); found != "" {
			event[pair.field] = normalizeDatadogEventField(pair.field, found)
		}
	}
	return event
}

func classifyDatadogOperationalKind(value map[string]any, attrs map[string]string) string {
	lower := strings.ToLower(strings.Join([]string{
		stringValue(value["type"]),
		stringValue(value["source"]),
		stringValue(value["kind"]),
		stringValue(value["resource_type"]),
		stringValue(value["urn"]),
		stringValue(value["title"]),
		stringValue(value["name"]),
		stringValue(value["message"]),
		attrs["resource_type"],
		attrs["event_type"],
		attrs["status"],
	}, " "))
	switch {
	case strings.Contains(lower, "notebook") || value["cells"] != nil:
		return "notebook"
	case strings.Contains(lower, "incident") || value["customer_impact"] != nil || value["root_cause"] != nil:
		return "incident"
	case strings.Contains(lower, "service_level_objective") || strings.Contains(lower, "slo") || value["target_threshold"] != nil || value["timeframe"] != nil:
		return "slo"
	case strings.Contains(lower, "monitor") || (value["query"] != nil && (value["message"] != nil || value["options"] != nil)):
		return "monitor"
	case strings.Contains(lower, "log") || value["ddsource"] != nil || value["hostname"] != nil || (value["message"] != nil && (value["status"] != nil || value["trace_id"] != nil)):
		return "log"
	default:
		return ""
	}
}

func normalizeDatadogEventField(field, value string) string {
	switch field {
	case "deploy":
		return ensurePrefix(value, "deploy:")
	case "migration":
		return ensurePrefix(value, "migration:")
	case "trace":
		return ensurePrefix(value, "trace:")
	case "commit":
		return ensurePrefix(value, "commit:")
	default:
		return value
	}
}

func parseEmbeddedJSONObject(value string) map[string]any {
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(value, "{") {
		return nil
	}
	var object map[string]any
	decoder := json.NewDecoder(strings.NewReader(value))
	decoder.UseNumber()
	if err := decoder.Decode(&object); err != nil {
		return nil
	}
	return object
}

func attrsFromDatadog(value map[string]any) map[string]string {
	attrs := map[string]string{}
	for _, field := range []string{"meta", "metrics", "attributes", "properties", "context"} {
		if nested, ok := value[field].(map[string]any); ok {
			for key, raw := range nested {
				if text := stringValue(raw); text != "" {
					attrs[key] = text
				}
			}
		}
	}
	if data, ok := value["data"].(map[string]any); ok {
		if nested, ok := data["attributes"].(map[string]any); ok {
			for key, raw := range nested {
				if text := stringValue(raw); text != "" {
					attrs[key] = text
				}
			}
		}
	}
	for key, raw := range value {
		if text := stringValue(raw); text != "" {
			attrs[key] = text
		}
	}
	if rawTags, ok := value["tags"]; ok {
		for key, val := range attrsFromTags(rawTags) {
			attrs[key] = val
		}
	}
	return attrs
}

func attrsFromTags(raw any) map[string]string {
	attrs := map[string]string{}
	items, ok := raw.([]any)
	if !ok {
		return attrs
	}
	for _, item := range items {
		tag := stringValue(item)
		key, value, ok := strings.Cut(tag, ":")
		if ok && key != "" && value != "" {
			attrs[key] = value
		}
	}
	return attrs
}

func adaptTraceSpan(source, traceID, spanID, name string, attrs map[string]string) ([]map[string]string, []string) {
	traceID = cleanID(traceID)
	spanID = cleanID(spanID)
	if traceID == "" || spanID == "" {
		return nil, nil
	}

	migrationID := firstNonEmpty(attrs["patchline.migration_id"], attrs["db.migration.id"], attrs["migration.id"])
	deployID := firstNonEmpty(attrs["patchline.deploy_id"], attrs["deployment.id"])
	commit := firstNonEmpty(attrs["git.commit.sha"], attrs["git.sha"], attrs["commit"])
	service := firstNonEmpty(attrs["service.name"], attrs["service"])
	statement := firstNonEmpty(attrs["db.statement"], attrs["db.query"], attrs["sql.query"], name)
	recordID := firstNonEmpty(attrs["patchline.record_id"], attrs["db.record.id"])
	traceEntity := "trace:" + traceID + "/" + spanID
	var events []map[string]string
	var warnings []string

	if migrationID == "" && statement != "" {
		return nil, []string{fmt.Sprintf("%s span %s has SQL evidence but no patchline.migration_id", source, spanID)}
	}
	if migrationID != "" {
		if deployID != "" && commit != "" && service != "" {
			events = append(events, map[string]string{
				"type":    "deploy",
				"id":      ensurePrefix(deployID, "deploy:"),
				"commit":  ensurePrefix(commit, "commit:"),
				"service": service,
				"source":  source,
			})
		}
		if deployID != "" {
			events = append(events, map[string]string{
				"type":   "migration",
				"id":     ensurePrefix(migrationID, "migration:"),
				"deploy": ensurePrefix(deployID, "deploy:"),
				"name":   firstNonEmpty(attrs["db.migration.name"], attrs["migration.name"]),
				"source": source,
			})
		}
		events = append(events, map[string]string{
			"type":      "trace",
			"id":        traceEntity,
			"migration": ensurePrefix(migrationID, "migration:"),
			"source":    source,
		})
	}
	if statement != "" && migrationID != "" {
		sqlEntity := "sql:" + spanID
		events = append(events, map[string]string{
			"type":        "sql_mutation",
			"id":          sqlEntity,
			"trace":       traceEntity,
			"fingerprint": normalizeWhitespace(statement),
			"source":      source,
		})
		if recordID != "" {
			events = append(events, map[string]string{
				"type":   "row_mutation",
				"record": ensurePrefix(recordID, "record:"),
				"sql":    sqlEntity,
				"source": source,
			})
		}
	}
	return events, warnings
}

func mergeAttrs(base, overlay map[string]string) map[string]string {
	merged := map[string]string{}
	for key, value := range base {
		merged[key] = value
	}
	for key, value := range overlay {
		merged[key] = value
	}
	return merged
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func ensurePrefix(value, prefix string) string {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, prefix) {
		return value
	}
	return prefix + value
}

func cleanID(value string) string {
	replacer := strings.NewReplacer(" ", "_", "\t", "_", "\n", "_", "\r", "_")
	return replacer.Replace(strings.TrimSpace(value))
}

func normalizeWhitespace(value string) string {
	return strings.Join(strings.Fields(value), " ")
}

func stringValue(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case json.Number:
		return typed.String()
	case float64:
		return fmt.Sprintf("%.0f", typed)
	case bool:
		return fmt.Sprintf("%t", typed)
	default:
		return ""
	}
}

func sortEvents(events []map[string]string) {
	sort.Slice(events, func(i, j int) bool {
		left := events[i]["type"] + "\x00" + events[i]["id"] + "\x00" + events[i]["record"] + "\x00" + events[i]["sql"]
		right := events[j]["type"] + "\x00" + events[j]["id"] + "\x00" + events[j]["record"] + "\x00" + events[j]["sql"]
		return left < right
	})
}

func dedupeEvents(events []map[string]string) []map[string]string {
	seen := map[string]bool{}
	out := make([]map[string]string, 0, len(events))
	for _, event := range events {
		key := canonical.Hash(event)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, event)
	}
	return out
}
