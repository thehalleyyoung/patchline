import * as crypto from "node:crypto";
import * as fs from "node:fs";
import * as path from "node:path";

const VERSION = "PLCI/1";
const EVIDENCE_TYPES = new Set(["source", "migration", "report", "telemetry", "spec"]);
const OBLIGATION_KINDS = new Set(["scope", "frame", "invariant", "rollback", "evidence", "interchange"]);
const STATUSES = new Set(["checked", "assumed", "refuted", "unsupported"]);
const VERDICTS = new Set(["safe", "guarded", "blocked", "unsupported"]);

type Evidence = { id: string; type: string; uri: string; sha256: string };
type Obligation = { id: string; kind: string; status: string; evidence: string[]; formula: string };
type Cert = {
  certificateId: string;
  subjectRepo: string;
  subjectRef: string;
  subjectPath: string;
  subjectSha256: string;
  issuedAt: string;
  producer: string;
  verdict: string;
  riskBps: number;
  canonicalSha256: string;
  evidence: Evidence[];
  obligations: Obligation[];
};

function validID(value: string): boolean {
  return /^[a-z][a-z0-9.-]{2,80}$/.test(value);
}

function validSHA256(value: string): boolean {
  return /^[0-9a-f]{64}$/.test(value);
}

function validLocalPath(value: string): boolean {
  return value.length > 0 && !value.startsWith("/") && !value.includes("\\") && value.split("/").every((part) => Boolean(part) && part !== "." && part !== ".." && /^[A-Za-z0-9_.-]+$/.test(part));
}

function sha256(data: Buffer | string): string {
  return crypto.createHash("sha256").update(data).digest("hex");
}

function lineError(index: number, message: string): Error {
  return new Error(`line ${index + 1}: ${message}`);
}

function parseField(lines: string[], state: { index: number }, prefix: string): string {
  if (state.index >= lines.length || !lines[state.index].startsWith(prefix)) {
    throw lineError(state.index, `expected ${prefix.slice(0, -2)} field`);
  }
  const value = lines[state.index].slice(prefix.length);
  if (!value) {
    throw lineError(state.index, `${prefix.slice(0, -2)} must not be empty`);
  }
  state.index += 1;
  return value;
}

function parseEvidenceLine(line: string, index: number): Evidence {
  const parts = line.slice("evidence: ".length).split(" ");
  if (parts.length !== 4 || parts.some((part) => part === "")) {
    throw lineError(index, "evidence line must have four single-spaced fields");
  }
  if (!parts[1].startsWith("type=") || !parts[2].startsWith("uri=") || !parts[3].startsWith("sha256=")) {
    throw lineError(index, "evidence attributes are out of order");
  }
  return {
    id: parts[0],
    type: parts[1].slice("type=".length),
    uri: parts[2].slice("uri=".length),
    sha256: parts[3].slice("sha256=".length),
  };
}

function parseObligationLine(line: string, index: number): Obligation {
  const rest = line.slice("obligation: ".length);
  const marker = ' formula="';
  const markerIndex = rest.indexOf(marker);
  if (markerIndex < 0 || !rest.endsWith('"')) {
    throw lineError(index, 'obligation must end with formula="..."');
  }
  const before = rest.slice(0, markerIndex);
  const formula = rest.slice(markerIndex + marker.length, -1);
  const parts = before.split(" ");
  if (parts.length !== 4 || parts.some((part) => part === "")) {
    throw lineError(index, "obligation line must have five single-spaced fields");
  }
  if (!parts[1].startsWith("kind=") || !parts[2].startsWith("status=") || !parts[3].startsWith("evidence=")) {
    throw lineError(index, "obligation attributes are out of order");
  }
  return {
    id: parts[0],
    kind: parts[1].slice("kind=".length),
    status: parts[2].slice("status=".length),
    evidence: parts[3].slice("evidence=".length).split(","),
    formula,
  };
}

function verifyFile(root: string, value: string, expected: string): void {
  if (!validLocalPath(value)) {
    throw new Error(`invalid local path ${JSON.stringify(value)}`);
  }
  const fullPath = path.join(root, ...value.split("/"));
  const got = sha256(fs.readFileSync(fullPath));
  if (got !== expected) {
    throw new Error(`${value} sha256 got ${got} want ${expected}`);
  }
}

function parseCertificate(filePath: string, root: string): void {
  const raw = fs.readFileSync(filePath);
  if (raw.length === 0) {
    throw new Error("empty certificate");
  }
  const rawText = raw.toString("utf8");
  if (rawText.includes("\r")) {
    throw new Error("PLCI certificates must use LF line endings");
  }
  if (!rawText.endsWith("\n")) {
    throw new Error("PLCI certificates must end with LF");
  }
  const lines = rawText.slice(0, -1).split("\n");
  if (lines.length < 14) {
    throw new Error("certificate is shorter than the PLCI/1 grammar minimum");
  }
  const state = { index: 0 };
  if (lines[state.index] !== VERSION) {
    throw lineError(state.index, "expected PLCI/1");
  }
  state.index += 1;
  const certificateId = parseField(lines, state, "certificate-id: ");
  const subjectRepo = parseField(lines, state, "subject-repo: ");
  const subjectRef = parseField(lines, state, "subject-ref: ");
  const subjectPath = parseField(lines, state, "subject-path: ");
  const subjectSha256 = parseField(lines, state, "subject-sha256: ");
  const issuedAt = parseField(lines, state, "issued-at: ");
  const producer = parseField(lines, state, "producer: ");
  const verdict = parseField(lines, state, "verdict: ");
  const riskBpsText = parseField(lines, state, "risk-bps: ");
  if (!/^(0|10000|[1-9][0-9]{0,3})$/.test(riskBpsText)) {
    throw new Error("risk-bps must be 0..10000 without leading zeros");
  }
  const riskBps = Number(riskBpsText);
  const evidence: Evidence[] = [];
  while (state.index < lines.length && lines[state.index].startsWith("evidence: ")) {
    evidence.push(parseEvidenceLine(lines[state.index], state.index));
    state.index += 1;
  }
  if (evidence.length === 0) {
    throw lineError(state.index, "expected at least one evidence line");
  }
  const obligations: Obligation[] = [];
  while (state.index < lines.length && lines[state.index].startsWith("obligation: ")) {
    obligations.push(parseObligationLine(lines[state.index], state.index));
    state.index += 1;
  }
  if (obligations.length === 0) {
    throw lineError(state.index, "expected at least one obligation line");
  }
  const canonicalIndex = state.index;
  const canonicalSha256 = parseField(lines, state, "canonical-sha256: ");
  if (state.index >= lines.length || lines[state.index] !== "END") {
    throw lineError(state.index, "expected END");
  }
  state.index += 1;
  if (state.index !== lines.length) {
    throw lineError(state.index, "unexpected trailing line");
  }
  const got = sha256(`${lines.slice(0, canonicalIndex).join("\n")}\n`);
  if (got !== canonicalSha256) {
    throw new Error(`canonical-sha256 mismatch: got ${canonicalSha256} want ${got}`);
  }
  validateCertificate({
    certificateId,
    subjectRepo,
    subjectRef,
    subjectPath,
    subjectSha256,
    issuedAt,
    producer,
    verdict,
    riskBps,
    canonicalSha256,
    evidence,
    obligations,
  }, root);
}

function validateCertificate(cert: Cert, root: string): void {
  if (!validID(cert.certificateId)) throw new Error(`invalid certificate-id ${JSON.stringify(cert.certificateId)}`);
  if (!/^[A-Za-z0-9_.-]+\/[A-Za-z0-9_.-]+$/.test(cert.subjectRepo)) throw new Error(`invalid subject-repo ${JSON.stringify(cert.subjectRepo)}`);
  if (!/^[A-Za-z0-9._/-]{1,80}$/.test(cert.subjectRef)) throw new Error(`invalid subject-ref ${JSON.stringify(cert.subjectRef)}`);
  if (!validLocalPath(cert.subjectPath)) throw new Error(`invalid subject-path ${JSON.stringify(cert.subjectPath)}`);
  if (!validSHA256(cert.subjectSha256) || !validSHA256(cert.canonicalSha256)) throw new Error("invalid sha256 field");
  if (!validIssuedAt(cert.issuedAt)) throw new Error(`invalid issued-at ${JSON.stringify(cert.issuedAt)}`);
  if (!validVChar(cert.producer)) throw new Error(`invalid producer ${JSON.stringify(cert.producer)}`);
  if (!VERDICTS.has(cert.verdict)) throw new Error(`invalid verdict ${JSON.stringify(cert.verdict)}`);
  if (cert.riskBps < 0 || cert.riskBps > 10000) throw new Error(`risk-bps out of range: ${cert.riskBps}`);
  verifyFile(root, cert.subjectPath, cert.subjectSha256);

  const evidenceIDs = new Set<string>();
  for (const item of cert.evidence) {
    if (!validID(item.id)) throw new Error(`invalid evidence id ${JSON.stringify(item.id)}`);
    if (evidenceIDs.has(item.id)) throw new Error(`duplicate evidence id ${JSON.stringify(item.id)}`);
    evidenceIDs.add(item.id);
    if (!EVIDENCE_TYPES.has(item.type)) throw new Error(`invalid evidence type ${JSON.stringify(item.type)}`);
    if (!validSHA256(item.sha256)) throw new Error(`invalid evidence sha256 for ${JSON.stringify(item.id)}`);
    if (item.uri.startsWith("file:")) {
      verifyFile(root, item.uri.slice("file:".length), item.sha256);
    } else if (!validRemoteURI(item.uri, "https://") && !validRemoteURI(item.uri, "repo://")) {
      throw new Error(`unsupported evidence uri ${JSON.stringify(item.uri)}`);
    }
  }

  const obligationIDs = new Set<string>();
  const statusCounts = new Map<string, number>();
  for (const item of cert.obligations) {
    if (!validID(item.id)) throw new Error(`invalid obligation id ${JSON.stringify(item.id)}`);
    if (obligationIDs.has(item.id)) throw new Error(`duplicate obligation id ${JSON.stringify(item.id)}`);
    obligationIDs.add(item.id);
    if (!OBLIGATION_KINDS.has(item.kind)) throw new Error(`invalid obligation kind ${JSON.stringify(item.kind)}`);
    if (!STATUSES.has(item.status)) throw new Error(`invalid obligation status ${JSON.stringify(item.status)}`);
    if (!validFormula(item.formula)) throw new Error(`invalid formula for obligation ${JSON.stringify(item.id)}`);
    statusCounts.set(item.status, (statusCounts.get(item.status) ?? 0) + 1);
    for (const ref of item.evidence) {
      if (!evidenceIDs.has(ref)) throw new Error(`obligation ${JSON.stringify(item.id)} references missing evidence ${JSON.stringify(ref)}`);
    }
  }
  const count = (status: string) => statusCounts.get(status) ?? 0;
  if (cert.verdict === "safe" && (count("assumed") > 0 || count("refuted") > 0 || count("unsupported") > 0)) {
    throw new Error("safe verdict cannot carry assumed, refuted, or unsupported obligations");
  }
  if (cert.verdict === "guarded" && (count("refuted") > 0 || count("unsupported") > 0)) {
    throw new Error("guarded verdict cannot carry refuted or unsupported obligations");
  }
  if (cert.verdict === "blocked" && count("refuted") === 0) {
    throw new Error("blocked verdict requires at least one refuted obligation");
  }
  if (cert.verdict === "unsupported" && count("unsupported") === 0) {
    throw new Error("unsupported verdict requires at least one unsupported obligation");
  }
}

function validIssuedAt(value: string): boolean {
  if (!/^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z$/.test(value)) return false;
  const year = Number(value.slice(0, 4));
  const month = Number(value.slice(5, 7));
  const day = Number(value.slice(8, 10));
  const hour = Number(value.slice(11, 13));
  const minute = Number(value.slice(14, 16));
  const second = Number(value.slice(17, 19));
  if (month < 1 || month > 12 || hour > 23 || minute > 59 || second > 59) return false;
  const days = [31, leapYear(year) ? 29 : 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31];
  return day >= 1 && day <= days[month - 1];
}

function leapYear(year: number): boolean {
  return year % 400 === 0 || (year % 4 === 0 && year % 100 !== 0);
}

function validRemoteURI(value: string, scheme: string): boolean {
  if (!value.startsWith(scheme)) return false;
  const rest = value.slice(scheme.length);
  return rest.length > 0 && /^[\x21-\x7E]+$/.test(rest);
}

function validVChar(value: string): boolean {
  return value.length > 0 && /^[\x21-\x7E]+$/.test(value);
}

function validFormula(value: string): boolean {
  return value.length > 0 && /^[\x20-\x21\x23-\x7E]+$/.test(value);
}

function checkDirectory(specDir: string, root: string) {
  const vectors = [];
  let accepted = 0;
  let rejected = 0;
  for (const [dirname, expected] of [["valid", "valid"], ["invalid", "invalid"]] as const) {
    const dir = path.join(specDir, "vectors", dirname);
    const files = fs.readdirSync(dir).filter((name) => name.endsWith(".plci")).sort();
    for (const file of files) {
      const filePath = path.join(dir, file);
      let isAccepted = false;
      let error: string | undefined;
      try {
        parseCertificate(filePath, root);
        isAccepted = true;
        accepted += 1;
      } catch (err) {
        error = err instanceof Error ? err.message : String(err);
        rejected += 1;
      }
      const ok = (expected === "valid" && isAccepted) || (expected === "invalid" && !isAccepted);
      const row: Record<string, string | boolean> = {
        path: path.relative(path.join(specDir, "vectors"), filePath).replaceAll(path.sep, "/"),
        expected,
        accepted: isAccepted,
        ok,
      };
      if (error) row.error = error;
      vectors.push(row);
    }
  }
  return {
    checker: "typescript",
    version: VERSION,
    spec_dir: specDir,
    total_valid: vectors.filter((row) => row.expected === "valid").length,
    total_invalid: vectors.filter((row) => row.expected === "invalid").length,
    accepted,
    rejected,
    all_ok: vectors.length > 0 && vectors.every((row) => row.ok === true),
    vectors,
  };
}

function main(): void {
  const args = process.argv.slice(2);
  let specDir = "specs/certificate-interchange/v1";
  let root = ".";
  let json = false;
  for (let i = 0; i < args.length; i += 1) {
    if (args[i] === "--spec-dir") specDir = args[++i];
    else if (args[i] === "--root") root = args[++i];
    else if (args[i] === "--json") json = true;
    else throw new Error(`unknown argument ${args[i]}`);
  }
  const report = checkDirectory(specDir, root);
  if (json) {
    console.log(JSON.stringify(report, null, 2));
  } else {
    console.log(`typescript PLCI/1 checker: valid=${report.total_valid} invalid=${report.total_invalid} all_ok=${report.all_ok}`);
  }
  process.exit(report.all_ok ? 0 : 1);
}

main();
