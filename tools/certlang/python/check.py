#!/usr/bin/env python3
import argparse
import hashlib
import json
import os
import re
from pathlib import Path

VERSION = "PLCI/1"
EVIDENCE_TYPES = {"source", "migration", "report", "telemetry", "spec"}
OBLIGATION_KINDS = {"scope", "frame", "invariant", "rollback", "evidence", "interchange"}
STATUSES = {"checked", "assumed", "refuted", "unsupported"}
VERDICTS = {"safe", "guarded", "blocked", "unsupported"}
ID_RE = re.compile(r"^[a-z][a-z0-9.-]{2,80}$")
ISSUED_RE = re.compile(r"^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z$")
REPO_RE = re.compile(r"^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$")
REF_RE = re.compile(r"^[A-Za-z0-9._/-]{1,80}$")


def valid_id(value):
    return bool(ID_RE.match(value))


def valid_sha256(value):
    return len(value) == 64 and all(ch in "0123456789abcdef" for ch in value)


def valid_local_path(value):
    if not value or value.startswith("/") or "\\" in value:
        return False
    parts = value.split("/")
    return all(part and part not in {".", ".."} and re.match(r"^[A-Za-z0-9_.-]+$", part) for part in parts)


def sha256_bytes(data):
    return hashlib.sha256(data).hexdigest()


def parse_field(lines, index, prefix):
    if index >= len(lines) or not lines[index].startswith(prefix):
        raise ValueError(f"line {index + 1}: expected {prefix[:-2]} field")
    value = lines[index][len(prefix):]
    if not value:
        raise ValueError(f"line {index + 1}: {prefix[:-2]} must not be empty")
    return value, index + 1


def parse_evidence(line, index):
    parts = line[len("evidence: "):].split(" ")
    if len(parts) != 4 or any(part == "" for part in parts):
        raise ValueError(f"line {index + 1}: evidence line must have four single-spaced fields")
    if not parts[1].startswith("type=") or not parts[2].startswith("uri=") or not parts[3].startswith("sha256="):
        raise ValueError(f"line {index + 1}: evidence attributes are out of order")
    return {
        "id": parts[0],
        "type": parts[1][5:],
        "uri": parts[2][4:],
        "sha256": parts[3][7:],
    }


def parse_obligation(line, index):
    rest = line[len("obligation: "):]
    marker = ' formula="'
    if marker not in rest or not rest.endswith('"'):
        raise ValueError(f"line {index + 1}: obligation must end with formula=\"...\"")
    before, formula = rest.split(marker, 1)
    formula = formula[:-1]
    parts = before.split(" ")
    if len(parts) != 4 or any(part == "" for part in parts):
        raise ValueError(f"line {index + 1}: obligation line must have five single-spaced fields")
    if not parts[1].startswith("kind=") or not parts[2].startswith("status=") or not parts[3].startswith("evidence="):
        raise ValueError(f"line {index + 1}: obligation attributes are out of order")
    return {
        "id": parts[0],
        "kind": parts[1][5:],
        "status": parts[2][7:],
        "evidence": parts[3][9:].split(","),
        "formula": formula,
    }


def verify_file(root, path, expected):
    if not valid_local_path(path):
        raise ValueError(f"invalid local path {path!r}")
    data = (root / Path(path)).read_bytes()
    got = sha256_bytes(data)
    if got != expected:
        raise ValueError(f"{path} sha256 got {got} want {expected}")


def parse_certificate(path, root):
    raw = path.read_bytes()
    if not raw:
        raise ValueError("empty certificate")
    if b"\r" in raw:
        raise ValueError("PLCI certificates must use LF line endings")
    if not raw.endswith(b"\n"):
        raise ValueError("PLCI certificates must end with LF")
    lines = raw.decode("utf-8").rstrip("\n").split("\n")
    if len(lines) < 14:
        raise ValueError("certificate is shorter than the PLCI/1 grammar minimum")
    index = 0
    if lines[index] != VERSION:
        raise ValueError("line 1: expected PLCI/1")
    index += 1
    cert = {}
    for key, prefix in [
        ("certificate_id", "certificate-id: "),
        ("subject_repo", "subject-repo: "),
        ("subject_ref", "subject-ref: "),
        ("subject_path", "subject-path: "),
        ("subject_sha256", "subject-sha256: "),
        ("issued_at", "issued-at: "),
        ("producer", "producer: "),
        ("verdict", "verdict: "),
        ("risk_bps", "risk-bps: "),
    ]:
        cert[key], index = parse_field(lines, index, prefix)
    if not re.match(r"^(0|10000|[1-9][0-9]{0,3})$", cert["risk_bps"]):
        raise ValueError("risk-bps must be 0..10000 without leading zeros")
    cert["risk_bps"] = int(cert["risk_bps"])

    evidence = []
    while index < len(lines) and lines[index].startswith("evidence: "):
        evidence.append(parse_evidence(lines[index], index))
        index += 1
    if not evidence:
        raise ValueError(f"line {index + 1}: expected at least one evidence line")
    obligations = []
    while index < len(lines) and lines[index].startswith("obligation: "):
        obligations.append(parse_obligation(lines[index], index))
        index += 1
    if not obligations:
        raise ValueError(f"line {index + 1}: expected at least one obligation line")
    canonical_index = index
    cert["canonical_sha256"], index = parse_field(lines, index, "canonical-sha256: ")
    if index >= len(lines) or lines[index] != "END":
        raise ValueError(f"line {index + 1}: expected END")
    index += 1
    if index != len(lines):
        raise ValueError(f"line {index + 1}: unexpected trailing line")

    canonical = ("\n".join(lines[:canonical_index]) + "\n").encode("utf-8")
    got = sha256_bytes(canonical)
    if cert["canonical_sha256"] != got:
        raise ValueError(f"canonical-sha256 mismatch: got {cert['canonical_sha256']} want {got}")

    validate_certificate(cert, evidence, obligations, root)


def validate_certificate(cert, evidence, obligations, root):
    if not valid_id(cert["certificate_id"]):
        raise ValueError(f"invalid certificate-id {cert['certificate_id']!r}")
    if not REPO_RE.match(cert["subject_repo"]):
        raise ValueError(f"invalid subject-repo {cert['subject_repo']!r}")
    if not REF_RE.match(cert["subject_ref"]):
        raise ValueError(f"invalid subject-ref {cert['subject_ref']!r}")
    if not valid_local_path(cert["subject_path"]):
        raise ValueError(f"invalid subject-path {cert['subject_path']!r}")
    if not valid_sha256(cert["subject_sha256"]):
        raise ValueError(f"invalid subject-sha256 {cert['subject_sha256']!r}")
    if not valid_issued_at(cert["issued_at"]):
        raise ValueError(f"invalid issued-at {cert['issued_at']!r}")
    if not valid_vchar(cert["producer"]):
        raise ValueError(f"invalid producer {cert['producer']!r}")
    if cert["verdict"] not in VERDICTS:
        raise ValueError(f"invalid verdict {cert['verdict']!r}")
    if cert["risk_bps"] < 0 or cert["risk_bps"] > 10000:
        raise ValueError(f"risk-bps out of range: {cert['risk_bps']}")
    verify_file(root, cert["subject_path"], cert["subject_sha256"])

    evidence_ids = set()
    for item in evidence:
        if not valid_id(item["id"]):
            raise ValueError(f"invalid evidence id {item['id']!r}")
        if item["id"] in evidence_ids:
            raise ValueError(f"duplicate evidence id {item['id']!r}")
        evidence_ids.add(item["id"])
        if item["type"] not in EVIDENCE_TYPES:
            raise ValueError(f"invalid evidence type {item['type']!r}")
        if not valid_sha256(item["sha256"]):
            raise ValueError(f"invalid evidence sha256 for {item['id']!r}")
        if item["uri"].startswith("file:"):
            verify_file(root, item["uri"][5:], item["sha256"])
        elif not (valid_remote_uri(item["uri"], "https://") or valid_remote_uri(item["uri"], "repo://")):
            raise ValueError(f"unsupported evidence uri {item['uri']!r}")

    status_counts = {status: 0 for status in STATUSES}
    obligation_ids = set()
    for item in obligations:
        if not valid_id(item["id"]):
            raise ValueError(f"invalid obligation id {item['id']!r}")
        if item["id"] in obligation_ids:
            raise ValueError(f"duplicate obligation id {item['id']!r}")
        obligation_ids.add(item["id"])
        if item["kind"] not in OBLIGATION_KINDS:
            raise ValueError(f"invalid obligation kind {item['kind']!r}")
        if item["status"] not in STATUSES:
            raise ValueError(f"invalid obligation status {item['status']!r}")
        if not valid_formula(item["formula"]):
            raise ValueError(f"invalid formula for obligation {item['id']!r}")
        status_counts[item["status"]] += 1
        for ref in item["evidence"]:
            if ref not in evidence_ids:
                raise ValueError(f"obligation {item['id']!r} references missing evidence {ref!r}")

    if cert["verdict"] == "safe" and any(status_counts[s] for s in ["assumed", "refuted", "unsupported"]):
        raise ValueError("safe verdict cannot carry assumed, refuted, or unsupported obligations")
    if cert["verdict"] == "guarded" and any(status_counts[s] for s in ["refuted", "unsupported"]):
        raise ValueError("guarded verdict cannot carry refuted or unsupported obligations")
    if cert["verdict"] == "blocked" and status_counts["refuted"] == 0:
        raise ValueError("blocked verdict requires at least one refuted obligation")
    if cert["verdict"] == "unsupported" and status_counts["unsupported"] == 0:
        raise ValueError("unsupported verdict requires at least one unsupported obligation")


def check_directory(spec_dir, root):
    vectors = []
    accepted = rejected = 0
    for dirname, expected in [("valid", "valid"), ("invalid", "invalid")]:
        for path in sorted((spec_dir / "vectors" / dirname).glob("*.plci")):
            error = None
            try:
                parse_certificate(path, root)
                is_accepted = True
                accepted += 1
            except Exception as exc:  # reported as checker output, not swallowed
                error = str(exc)
                is_accepted = False
                rejected += 1
            ok = (expected == "valid" and is_accepted) or (expected == "invalid" and not is_accepted)
            row = {
                "path": str(path.relative_to(spec_dir / "vectors")).replace(os.sep, "/"),
                "expected": expected,
                "accepted": is_accepted,
                "ok": ok,
            }
            if error:
                row["error"] = error
            vectors.append(row)
    return {
        "checker": "python",
        "version": VERSION,
        "spec_dir": str(spec_dir),
        "total_valid": sum(1 for row in vectors if row["expected"] == "valid"),
        "total_invalid": sum(1 for row in vectors if row["expected"] == "invalid"),
        "accepted": accepted,
        "rejected": rejected,
        "all_ok": bool(vectors) and all(row["ok"] for row in vectors),
        "vectors": vectors,
    }


def valid_issued_at(value):
    if not ISSUED_RE.match(value):
        return False
    year = int(value[0:4])
    month = int(value[5:7])
    day = int(value[8:10])
    hour = int(value[11:13])
    minute = int(value[14:16])
    second = int(value[17:19])
    if month < 1 or month > 12 or hour > 23 or minute > 59 or second > 59:
        return False
    days = [31, 29 if leap_year(year) else 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31]
    return 1 <= day <= days[month - 1]


def leap_year(year):
    return year % 400 == 0 or (year % 4 == 0 and year % 100 != 0)


def valid_remote_uri(value, scheme):
    if not value.startswith(scheme):
        return False
    rest = value[len(scheme):]
    return bool(rest) and all(0x21 <= ord(ch) <= 0x7E for ch in rest)


def valid_vchar(value):
    return bool(value) and all(0x21 <= ord(ch) <= 0x7E for ch in value)


def valid_formula(value):
    return bool(value) and all(ch != '"' and 0x20 <= ord(ch) <= 0x7E for ch in value)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--spec-dir", default="specs/certificate-interchange/v1")
    parser.add_argument("--root", default=".")
    parser.add_argument("--json", action="store_true")
    args = parser.parse_args()
    report = check_directory(Path(args.spec_dir), Path(args.root))
    if args.json:
        print(json.dumps(report, indent=2))
    else:
        print(f"python PLCI/1 checker: valid={report['total_valid']} invalid={report['total_invalid']} all_ok={report['all_ok']}")
    raise SystemExit(0 if report["all_ok"] else 1)


if __name__ == "__main__":
    main()
