use std::collections::{HashMap, HashSet};
use std::env;
use std::fs;
use std::path::{Path, PathBuf};

const VERSION: &str = "PLCI/1";

#[derive(Clone)]
struct Evidence {
    id: String,
    typ: String,
    uri: String,
    sha256: String,
}

#[derive(Clone)]
struct Obligation {
    id: String,
    kind: String,
    status: String,
    evidence: Vec<String>,
    formula: String,
}

struct Certificate {
    certificate_id: String,
    subject_repo: String,
    subject_ref: String,
    subject_path: String,
    subject_sha256: String,
    issued_at: String,
    producer: String,
    verdict: String,
    risk_bps: i64,
    canonical_sha256: String,
    evidence: Vec<Evidence>,
    obligations: Vec<Obligation>,
}

struct VectorResult {
    path: String,
    expected: String,
    accepted: bool,
    ok: bool,
    certificate_id: Option<String>,
    verdict: Option<String>,
    risk_bps: Option<i64>,
    canonical_sha256: Option<String>,
    error: Option<String>,
}

struct Report {
    vectors: Vec<VectorResult>,
    total_valid: usize,
    total_invalid: usize,
    accepted: usize,
    rejected: usize,
    all_ok: bool,
    spec_dir: String,
}

fn main() {
    let mut spec_dir = String::from("specs/certificate-interchange/v1");
    let mut root = String::from(".");
    let mut json = false;
    let mut args = env::args().skip(1);
    while let Some(arg) = args.next() {
        match arg.as_str() {
            "--spec-dir" => spec_dir = args.next().expect("--spec-dir requires a value"),
            "--root" => root = args.next().expect("--root requires a value"),
            "--json" => json = true,
            other => {
                eprintln!("unknown argument {other}");
                std::process::exit(2);
            }
        }
    }

    let report = check_directory(Path::new(&spec_dir), Path::new(&root));
    match report {
        Ok(report) => {
            if json {
                println!("{}", report_json(&report));
            } else {
                println!(
                    "rust PLCI/1 checker: valid={} invalid={} all_ok={}",
                    report.total_valid, report.total_invalid, report.all_ok
                );
            }
            if !report.all_ok {
                std::process::exit(1);
            }
        }
        Err(err) => {
            eprintln!("{err}");
            std::process::exit(1);
        }
    }
}

fn check_directory(spec_dir: &Path, root: &Path) -> Result<Report, String> {
    let mut vectors = Vec::new();
    let mut accepted = 0usize;
    let mut rejected = 0usize;
    for (dirname, expected) in [("valid", "valid"), ("invalid", "invalid")] {
        let dir = spec_dir.join("vectors").join(dirname);
        let mut paths = Vec::new();
        for entry in fs::read_dir(&dir).map_err(|err| format!("{}: {err}", dir.display()))? {
            let path = entry.map_err(|err| err.to_string())?.path();
            if path.extension().and_then(|s| s.to_str()) == Some("plci") {
                paths.push(path);
            }
        }
        paths.sort();
        for path in paths {
            let result = parse_certificate(&path, root);
            let (is_accepted, certificate_id, verdict, risk_bps, canonical_sha256, error) = match result {
                Ok(cert) => (
                    true,
                    Some(cert.certificate_id),
                    Some(cert.verdict),
                    Some(cert.risk_bps),
                    Some(cert.canonical_sha256),
                    None,
                ),
                Err(err) => (false, None, None, None, None, Some(err)),
            };
            if is_accepted {
                accepted += 1;
            } else {
                rejected += 1;
            }
            let ok = (expected == "valid" && is_accepted) || (expected == "invalid" && !is_accepted);
            let rel = path
                .strip_prefix(spec_dir.join("vectors"))
                .unwrap_or(&path)
                .to_string_lossy()
                .replace('\\', "/");
            vectors.push(VectorResult {
                path: rel,
                expected: expected.to_string(),
                accepted: is_accepted,
                ok,
                certificate_id,
                verdict,
                risk_bps,
                canonical_sha256,
                error,
            });
        }
    }
    let total_valid = vectors.iter().filter(|row| row.expected == "valid").count();
    let total_invalid = vectors.iter().filter(|row| row.expected == "invalid").count();
    let all_ok = total_valid > 0 && total_invalid > 0 && vectors.iter().all(|row| row.ok);
    Ok(Report {
        vectors,
        total_valid,
        total_invalid,
        accepted,
        rejected,
        all_ok,
        spec_dir: spec_dir.to_string_lossy().to_string(),
    })
}

fn parse_certificate(path: &Path, root: &Path) -> Result<Certificate, String> {
    let raw = fs::read(path).map_err(|err| format!("{}: {err}", path.display()))?;
    if raw.is_empty() {
        return Err("empty certificate".to_string());
    }
    if raw.iter().any(|byte| *byte == b'\r') {
        return Err("PLCI certificates must use LF line endings".to_string());
    }
    if !raw.ends_with(b"\n") {
        return Err("PLCI certificates must end with LF".to_string());
    }
    let text = String::from_utf8(raw).map_err(|err| err.to_string())?;
    let text = text.strip_suffix('\n').unwrap_or(&text);
    let lines: Vec<&str> = text.split('\n').collect();
    if lines.len() < 14 {
        return Err("certificate is shorter than the PLCI/1 grammar minimum".to_string());
    }
    let mut index = 0usize;
    expect_line(&lines, &mut index, VERSION)?;
    let certificate_id = parse_field(&lines, &mut index, "certificate-id: ")?;
    let subject_repo = parse_field(&lines, &mut index, "subject-repo: ")?;
    let subject_ref = parse_field(&lines, &mut index, "subject-ref: ")?;
    let subject_path = parse_field(&lines, &mut index, "subject-path: ")?;
    let subject_sha256 = parse_field(&lines, &mut index, "subject-sha256: ")?;
    let issued_at = parse_field(&lines, &mut index, "issued-at: ")?;
    let producer = parse_field(&lines, &mut index, "producer: ")?;
    let verdict = parse_field(&lines, &mut index, "verdict: ")?;
    let risk_bps_text = parse_field(&lines, &mut index, "risk-bps: ")?;
    if !valid_risk_bps_text(&risk_bps_text) {
        return Err(line_err(index, "risk-bps must be 0..10000 without leading zeros"));
    }
    let risk_bps = risk_bps_text
        .parse::<i64>()
        .map_err(|_| line_err(index, "risk-bps must be an integer"))?;

    let mut evidence = Vec::new();
    while index < lines.len() && lines[index].starts_with("evidence: ") {
        evidence.push(parse_evidence_line(lines[index], index)?);
        index += 1;
    }
    if evidence.is_empty() {
        return Err(line_err(index + 1, "expected at least one evidence line"));
    }
    let mut obligations = Vec::new();
    while index < lines.len() && lines[index].starts_with("obligation: ") {
        obligations.push(parse_obligation_line(lines[index], index)?);
        index += 1;
    }
    if obligations.is_empty() {
        return Err(line_err(index + 1, "expected at least one obligation line"));
    }
    let canonical_index = index;
    let canonical_sha256 = parse_field(&lines, &mut index, "canonical-sha256: ")?;
    expect_line(&lines, &mut index, "END")?;
    if index != lines.len() {
        return Err(line_err(index + 1, "unexpected trailing line"));
    }
    let canonical = format!("{}\n", lines[..canonical_index].join("\n"));
    let got = sha256_hex(canonical.as_bytes());
    if canonical_sha256 != got {
        return Err(format!(
            "canonical-sha256 mismatch: got {} want {}",
            canonical_sha256, got
        ));
    }
    let cert = Certificate {
        certificate_id,
        subject_repo,
        subject_ref,
        subject_path,
        subject_sha256,
        issued_at,
        producer,
        verdict,
        risk_bps,
        canonical_sha256,
        evidence,
        obligations,
    };
    validate_certificate(&cert, root)?;
    Ok(cert)
}

fn parse_field(lines: &[&str], index: &mut usize, prefix: &str) -> Result<String, String> {
    if *index >= lines.len() || !lines[*index].starts_with(prefix) {
        return Err(line_err(*index + 1, &format!("expected {} field", prefix.trim_end_matches(": "))));
    }
    let value = lines[*index][prefix.len()..].to_string();
    if value.is_empty() {
        return Err(line_err(*index + 1, &format!("{} must not be empty", prefix.trim_end_matches(": "))));
    }
    *index += 1;
    Ok(value)
}

fn expect_line(lines: &[&str], index: &mut usize, expected: &str) -> Result<(), String> {
    if *index >= lines.len() || lines[*index] != expected {
        return Err(line_err(*index + 1, &format!("expected {expected}")));
    }
    *index += 1;
    Ok(())
}

fn parse_evidence_line(line: &str, index: usize) -> Result<Evidence, String> {
    let parts: Vec<&str> = line["evidence: ".len()..].split(' ').collect();
    if parts.len() != 4 || parts.iter().any(|part| part.is_empty()) {
        return Err(line_err(index + 1, "evidence line must have four single-spaced fields"));
    }
    if !parts[1].starts_with("type=") || !parts[2].starts_with("uri=") || !parts[3].starts_with("sha256=") {
        return Err(line_err(index + 1, "evidence attributes are out of order"));
    }
    Ok(Evidence {
        id: parts[0].to_string(),
        typ: parts[1]["type=".len()..].to_string(),
        uri: parts[2]["uri=".len()..].to_string(),
        sha256: parts[3]["sha256=".len()..].to_string(),
    })
}

fn parse_obligation_line(line: &str, index: usize) -> Result<Obligation, String> {
    let rest = &line["obligation: ".len()..];
    let marker = " formula=\"";
    let pos = rest
        .find(marker)
        .ok_or_else(|| line_err(index + 1, "obligation must contain formula"))?;
    if !rest.ends_with('"') {
        return Err(line_err(index + 1, "obligation must end with quoted formula"));
    }
    let before = &rest[..pos];
    let formula = &rest[pos + marker.len()..rest.len() - 1];
    let parts: Vec<&str> = before.split(' ').collect();
    if parts.len() != 4 || parts.iter().any(|part| part.is_empty()) {
        return Err(line_err(index + 1, "obligation line must have five single-spaced fields"));
    }
    if !parts[1].starts_with("kind=") || !parts[2].starts_with("status=") || !parts[3].starts_with("evidence=") {
        return Err(line_err(index + 1, "obligation attributes are out of order"));
    }
    Ok(Obligation {
        id: parts[0].to_string(),
        kind: parts[1]["kind=".len()..].to_string(),
        status: parts[2]["status=".len()..].to_string(),
        evidence: parts[3]["evidence=".len()..]
            .split(',')
            .map(|s| s.to_string())
            .collect(),
        formula: formula.to_string(),
    })
}

fn validate_certificate(cert: &Certificate, root: &Path) -> Result<(), String> {
    if !valid_id(&cert.certificate_id) {
        return Err(format!("invalid certificate-id {:?}", cert.certificate_id));
    }
    if !valid_repo(&cert.subject_repo) {
        return Err(format!("invalid subject-repo {:?}", cert.subject_repo));
    }
    if !valid_ref(&cert.subject_ref) {
        return Err(format!("invalid subject-ref {:?}", cert.subject_ref));
    }
    if !valid_local_path(&cert.subject_path) {
        return Err(format!("invalid subject-path {:?}", cert.subject_path));
    }
    if !valid_sha256(&cert.subject_sha256) || !valid_sha256(&cert.canonical_sha256) {
        return Err("invalid sha256 field".to_string());
    }
    if !valid_issued_at(&cert.issued_at) {
        return Err(format!("invalid issued-at {:?}", cert.issued_at));
    }
    if !valid_vchar(&cert.producer) {
        return Err(format!("invalid producer {:?}", cert.producer));
    }
    if !contains(&["safe", "guarded", "blocked", "unsupported"], &cert.verdict) {
        return Err(format!("invalid verdict {:?}", cert.verdict));
    }
    if cert.risk_bps < 0 || cert.risk_bps > 10000 {
        return Err(format!("risk-bps out of range: {}", cert.risk_bps));
    }
    verify_file(root, &cert.subject_path, &cert.subject_sha256)?;

    let mut evidence_ids = HashSet::new();
    for item in &cert.evidence {
        if !valid_id(&item.id) {
            return Err(format!("invalid evidence id {:?}", item.id));
        }
        if !evidence_ids.insert(item.id.clone()) {
            return Err(format!("duplicate evidence id {:?}", item.id));
        }
        if !contains(&["source", "migration", "report", "telemetry", "spec"], &item.typ) {
            return Err(format!("invalid evidence type {:?}", item.typ));
        }
        if !valid_sha256(&item.sha256) {
            return Err(format!("invalid evidence sha256 for {:?}", item.id));
        }
        if let Some(path) = item.uri.strip_prefix("file:") {
            verify_file(root, path, &item.sha256)?;
        } else if !(valid_remote_uri(&item.uri, "https://") || valid_remote_uri(&item.uri, "repo://")) {
            return Err(format!("unsupported evidence uri {:?}", item.uri));
        }
    }

    let mut obligation_ids = HashSet::new();
    let mut status_counts: HashMap<String, usize> = HashMap::new();
    for item in &cert.obligations {
        if !valid_id(&item.id) {
            return Err(format!("invalid obligation id {:?}", item.id));
        }
        if !obligation_ids.insert(item.id.clone()) {
            return Err(format!("duplicate obligation id {:?}", item.id));
        }
        if !contains(&["scope", "frame", "invariant", "rollback", "evidence", "interchange"], &item.kind) {
            return Err(format!("invalid obligation kind {:?}", item.kind));
        }
        if !contains(&["checked", "assumed", "refuted", "unsupported"], &item.status) {
            return Err(format!("invalid obligation status {:?}", item.status));
        }
        if !valid_formula(&item.formula) {
            return Err(format!("invalid formula for obligation {:?}", item.id));
        }
        *status_counts.entry(item.status.clone()).or_insert(0) += 1;
        for evidence in &item.evidence {
            if !evidence_ids.contains(evidence) {
                return Err(format!(
                    "obligation {:?} references missing evidence {:?}",
                    item.id, evidence
                ));
            }
        }
    }

    let count = |status: &str| -> usize { *status_counts.get(status).unwrap_or(&0) };
    match cert.verdict.as_str() {
        "safe" if count("assumed") > 0 || count("refuted") > 0 || count("unsupported") > 0 => {
            Err("safe verdict cannot carry assumed, refuted, or unsupported obligations".to_string())
        }
        "guarded" if count("refuted") > 0 || count("unsupported") > 0 => {
            Err("guarded verdict cannot carry refuted or unsupported obligations".to_string())
        }
        "blocked" if count("refuted") == 0 => {
            Err("blocked verdict requires at least one refuted obligation".to_string())
        }
        "unsupported" if count("unsupported") == 0 => {
            Err("unsupported verdict requires at least one unsupported obligation".to_string())
        }
        _ => Ok(()),
    }
}

fn verify_file(root: &Path, value: &str, expected: &str) -> Result<(), String> {
    if !valid_local_path(value) {
        return Err(format!("invalid local path {:?}", value));
    }
    let mut path = PathBuf::from(root);
    for part in value.split('/') {
        path.push(part);
    }
    let data = fs::read(&path).map_err(|err| format!("{}: {err}", path.display()))?;
    let got = sha256_hex(&data);
    if got != expected {
        return Err(format!("{} sha256 got {} want {}", value, got, expected));
    }
    Ok(())
}

fn valid_id(value: &str) -> bool {
    let bytes = value.as_bytes();
    if bytes.len() < 3 || bytes.len() > 81 || !bytes[0].is_ascii_lowercase() {
        return false;
    }
    bytes
        .iter()
        .all(|b| b.is_ascii_lowercase() || b.is_ascii_digit() || *b == b'.' || *b == b'-')
}

fn valid_sha256(value: &str) -> bool {
    value.len() == 64 && value.bytes().all(|b| b.is_ascii_digit() || (b'a'..=b'f').contains(&b))
}

fn valid_risk_bps_text(value: &str) -> bool {
    if value == "0" || value == "10000" {
        return true;
    }
    let bytes = value.as_bytes();
    !bytes.is_empty()
        && bytes.len() <= 4
        && (b'1'..=b'9').contains(&bytes[0])
        && bytes[1..].iter().all(|b| b.is_ascii_digit())
}

fn valid_repo(value: &str) -> bool {
    let parts: Vec<&str> = value.split('/').collect();
    parts.len() == 2
        && parts
            .iter()
            .all(|part| !part.is_empty() && part.bytes().all(|b| b.is_ascii_alphanumeric() || b == b'_' || b == b'.' || b == b'-'))
}

fn valid_ref(value: &str) -> bool {
    !value.is_empty()
        && value.len() <= 80
        && value
            .bytes()
            .all(|b| b.is_ascii_alphanumeric() || b == b'_' || b == b'.' || b == b'-' || b == b'/')
}

fn valid_issued_at(value: &str) -> bool {
    let bytes = value.as_bytes();
    if bytes.len() != 20 {
        return false;
    }
    for (index, byte) in bytes.iter().enumerate() {
        match index {
            4 | 7 => {
                if *byte != b'-' {
                    return false;
                }
            }
            10 => {
                if *byte != b'T' {
                    return false;
                }
            }
            13 | 16 => {
                if *byte != b':' {
                    return false;
                }
            }
            19 => {
                if *byte != b'Z' {
                    return false;
                }
            }
            _ => {
                if !byte.is_ascii_digit() {
                    return false;
                }
            }
        }
    }
    let year = value[0..4].parse::<i32>().unwrap_or(-1);
    let month = value[5..7].parse::<usize>().unwrap_or(0);
    let day = value[8..10].parse::<usize>().unwrap_or(0);
    let hour = value[11..13].parse::<usize>().unwrap_or(99);
    let minute = value[14..16].parse::<usize>().unwrap_or(99);
    let second = value[17..19].parse::<usize>().unwrap_or(99);
    if month == 0 || month > 12 || hour > 23 || minute > 59 || second > 59 {
        return false;
    }
    let mut days = [31usize, 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31];
    if leap_year(year) {
        days[1] = 29;
    }
    day >= 1 && day <= days[month - 1]
}

fn leap_year(year: i32) -> bool {
    year % 400 == 0 || (year % 4 == 0 && year % 100 != 0)
}

fn valid_local_path(value: &str) -> bool {
    if value.is_empty() || value.starts_with('/') || value.contains('\\') {
        return false;
    }
    value
        .split('/')
        .all(|part| {
            !part.is_empty()
                && part != "."
                && part != ".."
                && part
                    .bytes()
                    .all(|b| b.is_ascii_alphanumeric() || b == b'_' || b == b'.' || b == b'-')
        })
}

fn valid_remote_uri(value: &str, scheme: &str) -> bool {
    if !value.starts_with(scheme) {
        return false;
    }
    let rest = &value[scheme.len()..];
    !rest.is_empty() && rest.bytes().all(|b| (0x21..=0x7e).contains(&b))
}

fn valid_vchar(value: &str) -> bool {
    !value.is_empty() && value.bytes().all(|b| (0x21..=0x7e).contains(&b))
}

fn valid_formula(value: &str) -> bool {
    !value.is_empty() && value.bytes().all(|b| b != b'"' && (0x20..=0x7e).contains(&b))
}

fn contains(values: &[&str], value: &str) -> bool {
    values.iter().any(|candidate| *candidate == value)
}

fn line_err(line: usize, message: &str) -> String {
    format!("line {line}: {message}")
}

fn report_json(report: &Report) -> String {
    let mut out = String::new();
    out.push_str("{\n");
    out.push_str("  \"checker\": \"rust\",\n");
    out.push_str(&format!("  \"version\": \"{}\",\n", VERSION));
    out.push_str(&format!("  \"spec_dir\": \"{}\",\n", json_escape(&report.spec_dir)));
    out.push_str(&format!("  \"total_valid\": {},\n", report.total_valid));
    out.push_str(&format!("  \"total_invalid\": {},\n", report.total_invalid));
    out.push_str(&format!("  \"accepted\": {},\n", report.accepted));
    out.push_str(&format!("  \"rejected\": {},\n", report.rejected));
    out.push_str(&format!("  \"all_ok\": {},\n", report.all_ok));
    out.push_str("  \"vectors\": [\n");
    for (index, row) in report.vectors.iter().enumerate() {
        out.push_str("    {\n");
        out.push_str(&format!("      \"path\": \"{}\",\n", json_escape(&row.path)));
        out.push_str(&format!("      \"expected\": \"{}\",\n", json_escape(&row.expected)));
        out.push_str(&format!("      \"accepted\": {},\n", row.accepted));
        out.push_str(&format!("      \"ok\": {}", row.ok));
        if let Some(value) = &row.certificate_id {
            out.push_str(&format!(",\n      \"certificate_id\": \"{}\"", json_escape(value)));
        }
        if let Some(value) = &row.verdict {
            out.push_str(&format!(",\n      \"verdict\": \"{}\"", json_escape(value)));
        }
        if let Some(value) = row.risk_bps {
            out.push_str(&format!(",\n      \"risk_bps\": {}", value));
        }
        if let Some(value) = &row.canonical_sha256 {
            out.push_str(&format!(",\n      \"canonical_sha256\": \"{}\"", json_escape(value)));
        }
        if let Some(err) = &row.error {
            out.push_str(&format!(",\n      \"error\": \"{}\"", json_escape(err)));
        }
        out.push_str("\n    }");
        if index + 1 != report.vectors.len() {
            out.push(',');
        }
        out.push('\n');
    }
    out.push_str("  ]\n");
    out.push('}');
    out
}

fn json_escape(value: &str) -> String {
    let mut out = String::new();
    for ch in value.chars() {
        match ch {
            '"' => out.push_str("\\\""),
            '\\' => out.push_str("\\\\"),
            '\n' => out.push_str("\\n"),
            '\r' => out.push_str("\\r"),
            '\t' => out.push_str("\\t"),
            other => out.push(other),
        }
    }
    out
}

fn sha256_hex(data: &[u8]) -> String {
    let digest = sha256(data);
    let mut out = String::with_capacity(64);
    for byte in digest {
        out.push_str(&format!("{:02x}", byte));
    }
    out
}

fn sha256(data: &[u8]) -> [u8; 32] {
    const K: [u32; 64] = [
        0x428a2f98, 0x71374491, 0xb5c0fbcf, 0xe9b5dba5, 0x3956c25b, 0x59f111f1, 0x923f82a4,
        0xab1c5ed5, 0xd807aa98, 0x12835b01, 0x243185be, 0x550c7dc3, 0x72be5d74, 0x80deb1fe,
        0x9bdc06a7, 0xc19bf174, 0xe49b69c1, 0xefbe4786, 0x0fc19dc6, 0x240ca1cc, 0x2de92c6f,
        0x4a7484aa, 0x5cb0a9dc, 0x76f988da, 0x983e5152, 0xa831c66d, 0xb00327c8, 0xbf597fc7,
        0xc6e00bf3, 0xd5a79147, 0x06ca6351, 0x14292967, 0x27b70a85, 0x2e1b2138, 0x4d2c6dfc,
        0x53380d13, 0x650a7354, 0x766a0abb, 0x81c2c92e, 0x92722c85, 0xa2bfe8a1, 0xa81a664b,
        0xc24b8b70, 0xc76c51a3, 0xd192e819, 0xd6990624, 0xf40e3585, 0x106aa070, 0x19a4c116,
        0x1e376c08, 0x2748774c, 0x34b0bcb5, 0x391c0cb3, 0x4ed8aa4a, 0x5b9cca4f, 0x682e6ff3,
        0x748f82ee, 0x78a5636f, 0x84c87814, 0x8cc70208, 0x90befffa, 0xa4506ceb, 0xbef9a3f7,
        0xc67178f2,
    ];
    let mut h: [u32; 8] = [
        0x6a09e667,
        0xbb67ae85,
        0x3c6ef372,
        0xa54ff53a,
        0x510e527f,
        0x9b05688c,
        0x1f83d9ab,
        0x5be0cd19,
    ];
    let bit_len = (data.len() as u64) * 8;
    let mut padded = data.to_vec();
    padded.push(0x80);
    while (padded.len() + 8) % 64 != 0 {
        padded.push(0);
    }
    padded.extend_from_slice(&bit_len.to_be_bytes());

    for chunk in padded.chunks_exact(64) {
        let mut w = [0u32; 64];
        for i in 0..16 {
            let j = i * 4;
            w[i] = u32::from_be_bytes([chunk[j], chunk[j + 1], chunk[j + 2], chunk[j + 3]]);
        }
        for i in 16..64 {
            let s0 = w[i - 15].rotate_right(7) ^ w[i - 15].rotate_right(18) ^ (w[i - 15] >> 3);
            let s1 = w[i - 2].rotate_right(17) ^ w[i - 2].rotate_right(19) ^ (w[i - 2] >> 10);
            w[i] = w[i - 16]
                .wrapping_add(s0)
                .wrapping_add(w[i - 7])
                .wrapping_add(s1);
        }
        let mut a = h[0];
        let mut b = h[1];
        let mut c = h[2];
        let mut d = h[3];
        let mut e = h[4];
        let mut f = h[5];
        let mut g = h[6];
        let mut hh = h[7];
        for i in 0..64 {
            let s1 = e.rotate_right(6) ^ e.rotate_right(11) ^ e.rotate_right(25);
            let ch = (e & f) ^ ((!e) & g);
            let temp1 = hh
                .wrapping_add(s1)
                .wrapping_add(ch)
                .wrapping_add(K[i])
                .wrapping_add(w[i]);
            let s0 = a.rotate_right(2) ^ a.rotate_right(13) ^ a.rotate_right(22);
            let maj = (a & b) ^ (a & c) ^ (b & c);
            let temp2 = s0.wrapping_add(maj);
            hh = g;
            g = f;
            f = e;
            e = d.wrapping_add(temp1);
            d = c;
            c = b;
            b = a;
            a = temp1.wrapping_add(temp2);
        }
        h[0] = h[0].wrapping_add(a);
        h[1] = h[1].wrapping_add(b);
        h[2] = h[2].wrapping_add(c);
        h[3] = h[3].wrapping_add(d);
        h[4] = h[4].wrapping_add(e);
        h[5] = h[5].wrapping_add(f);
        h[6] = h[6].wrapping_add(g);
        h[7] = h[7].wrapping_add(hh);
    }

    let mut out = [0u8; 32];
    for (index, value) in h.iter().enumerate() {
        out[index * 4..index * 4 + 4].copy_from_slice(&value.to_be_bytes());
    }
    out
}
