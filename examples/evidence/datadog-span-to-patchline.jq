def attr($key):
  (.attributes[$key] // .meta[$key] // .tags[$key] // empty);

if attr("patchline.event_type") == "trace" then
  {
    type: "trace",
    id: ("trace:" + (.trace_id | tostring)),
    migration: attr("patchline.migration_id")
  }
elif attr("patchline.event_type") == "sql_mutation" then
  {
    type: "sql_mutation",
    id: ("sql:" + (attr("db.statement.fingerprint") // .span_id | tostring)),
    trace: ("trace:" + (.trace_id | tostring)),
    fingerprint: attr("db.statement.fingerprint")
  }
else
  empty
end
