package offlinedeploy

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"github.com/thehalleyyoung/patchline/internal/canonical"
)

const SpecVersion = "patchline.offline-deploy/v1"
const ReportVersion = "patchline.offline-deploy-report/v1"

type Spec struct {
	Version  string    `json:"version"`
	Name     string    `json:"name"`
	Claim    string    `json:"claim,omitempty"`
	Criteria Criteria  `json:"criteria"`
	Profiles []Profile `json:"profiles"`
}

type Criteria struct {
	RequiredEnvironments            []string `json:"required_environments"`
	MinProfiles                     int      `json:"min_profiles"`
	MinBundlesPerProfile            int      `json:"min_bundles_per_profile"`
	MinUpdateBundlesPerProfile      int      `json:"min_update_bundles_per_profile"`
	MinRegulatoryControlsPerProfile int      `json:"min_regulatory_controls_per_profile"`
	RequireNoNetwork                bool     `json:"require_no_network"`
	RequireTelemetryDisabled        bool     `json:"require_telemetry_disabled"`
	RequirePinnedBundles            bool     `json:"require_pinned_bundles"`
	RequirePinnedUpdateBundles      bool     `json:"require_pinned_update_bundles"`
	RequireOfflineUpdates           bool     `json:"require_offline_updates"`
	RequireReproducibleCommands     bool     `json:"require_reproducible_commands"`
	RequireRollbackPlan             bool     `json:"require_rollback_plan"`
	RequireEvidenceHashes           bool     `json:"require_evidence_hashes"`
	RequireBundleSignatures         bool     `json:"require_bundle_signatures"`
	RequireSoftwareBillOfMaterials  bool     `json:"require_software_bill_of_materials"`
}

type Profile struct {
	ID                 string          `json:"id"`
	Environment        string          `json:"environment"`
	Site               string          `json:"site"`
	InstallTarget      string          `json:"install_target"`
	RegulatoryControls []string        `json:"regulatory_controls"`
	NetworkPolicy      NetworkPolicy   `json:"network_policy"`
	TelemetryPolicy    TelemetryPolicy `json:"telemetry_policy"`
	InstallCommands    []string        `json:"install_commands"`
	VerifyCommands     []string        `json:"verify_commands"`
	EvidencePaths      []string        `json:"evidence_paths"`
	Bundles            []Bundle        `json:"bundles"`
	UpdateBundles      []UpdateBundle  `json:"update_bundles"`
	RollbackPlan       RollbackPlan    `json:"rollback_plan"`
}

type NetworkPolicy struct {
	Mode             string   `json:"mode"`
	EgressAllowed    bool     `json:"egress_allowed"`
	AllowedEndpoints []string `json:"allowed_endpoints,omitempty"`
}

type TelemetryPolicy struct {
	Mode         string   `json:"mode"`
	Enabled      bool     `json:"enabled"`
	Destinations []string `json:"destinations,omitempty"`
	LocalOnly    bool     `json:"local_only,omitempty"`
}

type Bundle struct {
	ID            string `json:"id"`
	Kind          string `json:"kind"`
	Version       string `json:"version"`
	Path          string `json:"path"`
	SHA256        string `json:"sha256"`
	SignaturePath string `json:"signature_path"`
	SBOMPath      string `json:"sbom_path"`
	UpdateChannel string `json:"update_channel"`
}

type UpdateBundle struct {
	ID            string   `json:"id"`
	FromVersion   string   `json:"from_version"`
	ToVersion     string   `json:"to_version"`
	Path          string   `json:"path"`
	SHA256        string   `json:"sha256"`
	ManifestPath  string   `json:"manifest_path"`
	SignaturePath string   `json:"signature_path"`
	Offline       bool     `json:"offline"`
	AppliesTo     []string `json:"applies_to"`
}

type RollbackPlan struct {
	ID                   string   `json:"id"`
	MaxMinutes           int      `json:"max_minutes"`
	PreviousBundleSHA256 string   `json:"previous_bundle_sha256"`
	Commands             []string `json:"commands"`
	EvidencePaths        []string `json:"evidence_paths"`
}

type Report struct {
	Version         string           `json:"version"`
	Name            string           `json:"name"`
	OK              bool             `json:"ok"`
	Criteria        Criteria         `json:"criteria"`
	Summary         Summary          `json:"summary"`
	Profiles        []ProfileReport  `json:"profiles"`
	Counterexamples []Counterexample `json:"counterexamples,omitempty"`
	Hash            string           `json:"hash"`
}

type Summary struct {
	Profiles                  int `json:"profiles"`
	NoNetworkProfiles         int `json:"no_network_profiles"`
	TelemetryDisabledProfiles int `json:"telemetry_disabled_profiles"`
	Bundles                   int `json:"bundles"`
	PinnedBundles             int `json:"pinned_bundles"`
	UpdateBundles             int `json:"update_bundles"`
	OfflineUpdateBundles      int `json:"offline_update_bundles"`
	RollbackPlans             int `json:"rollback_plans"`
	ReproducibleCommands      int `json:"reproducible_commands"`
	RegulatoryControls        int `json:"regulatory_controls"`
	EvidenceArtifacts         int `json:"evidence_artifacts"`
	Counterexamples           int `json:"counterexamples"`
}

type ProfileReport struct {
	ID                   string             `json:"id"`
	Environment          string             `json:"environment"`
	Site                 string             `json:"site"`
	InstallTarget        string             `json:"install_target"`
	RegulatoryControls   []string           `json:"regulatory_controls"`
	NoNetwork            bool               `json:"no_network"`
	TelemetryDisabled    bool               `json:"telemetry_disabled"`
	ReproducibleCommands int                `json:"reproducible_commands"`
	Evidence             []ArtifactEvidence `json:"evidence"`
	Bundles              []BundleReport     `json:"bundles"`
	UpdateBundles        []UpdateReport     `json:"update_bundles"`
	Rollback             RollbackReport     `json:"rollback"`
}

type BundleReport struct {
	ID        string           `json:"id"`
	Kind      string           `json:"kind"`
	Version   string           `json:"version"`
	Channel   string           `json:"channel"`
	Artifact  ArtifactEvidence `json:"artifact"`
	Expected  string           `json:"expected_sha256"`
	Pinned    bool             `json:"pinned"`
	Signature ArtifactEvidence `json:"signature,omitempty"`
	SBOM      ArtifactEvidence `json:"sbom,omitempty"`
}

type UpdateReport struct {
	ID        string           `json:"id"`
	From      string           `json:"from_version"`
	To        string           `json:"to_version"`
	Offline   bool             `json:"offline"`
	AppliesTo []string         `json:"applies_to"`
	Artifact  ArtifactEvidence `json:"artifact"`
	Expected  string           `json:"expected_sha256"`
	Manifest  ArtifactEvidence `json:"manifest,omitempty"`
	Signature ArtifactEvidence `json:"signature,omitempty"`
}

type RollbackReport struct {
	ID                   string             `json:"id,omitempty"`
	Present              bool               `json:"present"`
	MaxMinutes           int                `json:"max_minutes,omitempty"`
	PreviousBundleSHA256 string             `json:"previous_bundle_sha256,omitempty"`
	Commands             int                `json:"commands"`
	Evidence             []ArtifactEvidence `json:"evidence,omitempty"`
}

type ArtifactEvidence struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Bytes  int64  `json:"bytes"`
}

type Counterexample struct {
	ID      string   `json:"id"`
	Kind    string   `json:"kind"`
	Subject string   `json:"subject,omitempty"`
	Message string   `json:"message"`
	Witness []string `json:"witness,omitempty"`
}

func ReadSpec(reader io.Reader) (Spec, error) {
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	var spec Spec
	if err := decoder.Decode(&spec); err != nil {
		return Spec{}, err
	}
	if spec.Version != SpecVersion {
		return Spec{}, fmt.Errorf("offline deploy spec version must be %s", SpecVersion)
	}
	return spec, nil
}

func BuildReport(spec Spec, root string) (Report, error) {
	if err := validateSpec(spec); err != nil {
		return Report{}, err
	}
	if root == "" {
		root = "."
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return Report{}, err
	}
	criteria := normalizeCriteria(spec.Criteria)
	report := Report{
		Version:  ReportVersion,
		Name:     spec.Name,
		OK:       true,
		Criteria: criteria,
	}
	environments := map[string]struct{}{}
	for _, profile := range sortedProfiles(spec.Profiles) {
		pr, counterexamples := buildProfileReport(rootAbs, profile, criteria)
		report.Profiles = append(report.Profiles, pr)
		report.Counterexamples = append(report.Counterexamples, counterexamples...)
		if env := normalizeToken(profile.Environment); env != "" {
			environments[env] = struct{}{}
		}
		report.Summary.Profiles++
		report.Summary.Bundles += len(pr.Bundles)
		report.Summary.UpdateBundles += len(pr.UpdateBundles)
		report.Summary.ReproducibleCommands += pr.ReproducibleCommands
		report.Summary.RegulatoryControls += len(pr.RegulatoryControls)
		report.Summary.EvidenceArtifacts += len(pr.Evidence) + len(pr.Rollback.Evidence)
		for _, bundle := range pr.Bundles {
			if bundle.Pinned {
				report.Summary.PinnedBundles++
			}
			if bundle.Signature.Path != "" {
				report.Summary.EvidenceArtifacts++
			}
			if bundle.SBOM.Path != "" {
				report.Summary.EvidenceArtifacts++
			}
		}
		for _, update := range pr.UpdateBundles {
			if update.Offline && update.Artifact.Path != "" && update.Artifact.SHA256 == update.Expected {
				report.Summary.OfflineUpdateBundles++
			}
			if update.Manifest.Path != "" {
				report.Summary.EvidenceArtifacts++
			}
			if update.Signature.Path != "" {
				report.Summary.EvidenceArtifacts++
			}
		}
		if pr.NoNetwork {
			report.Summary.NoNetworkProfiles++
		}
		if pr.TelemetryDisabled {
			report.Summary.TelemetryDisabledProfiles++
		}
		if pr.Rollback.Present {
			report.Summary.RollbackPlans++
		}
	}
	if len(spec.Profiles) < criteria.MinProfiles {
		report.Counterexamples = append(report.Counterexamples, Counterexample{
			ID:      "criteria.profiles.insufficient",
			Kind:    "insufficient_profiles",
			Message: fmt.Sprintf("profiles %d below required %d", len(spec.Profiles), criteria.MinProfiles),
		})
	}
	for _, required := range criteria.RequiredEnvironments {
		if _, ok := environments[required]; !ok {
			report.Counterexamples = append(report.Counterexamples, Counterexample{
				ID:      "environment." + stableID(required, "required") + ".missing",
				Kind:    "missing_required_environment",
				Subject: required,
				Message: "required regulated deployment environment is not covered",
				Witness: []string{required},
			})
		}
	}
	sortCounterexamples(report.Counterexamples)
	report.Summary.Counterexamples = len(report.Counterexamples)
	report.OK = len(report.Counterexamples) == 0
	report.Hash = reportHash(report)
	return report, nil
}

func WriteArtifacts(outDir string, report Report) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	file, err := os.Create(filepath.Join(outDir, "offline-deploy.json"))
	if err != nil {
		return err
	}
	if err := canonical.WriteJSON(file, report); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(outDir, "offline-deploy.md"), []byte(RenderMarkdown(report)), 0o644)
}

func RenderMarkdown(report Report) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Reproducible edge/offline deployment\n\n")
	fmt.Fprintf(&b, "Patchline validates regulated edge deployments as pinned, self-contained profiles: no network egress, no telemetry destinations, content-addressed install and update bundles, reproducible local commands, and explicit rollback evidence.\n\n")
	fmt.Fprintf(&b, "## Summary\n\n")
	fmt.Fprintf(&b, "| Metric | Value |\n| --- | ---: |\n")
	fmt.Fprintf(&b, "| OK | `%t` |\n", report.OK)
	fmt.Fprintf(&b, "| Profiles | %d |\n", report.Summary.Profiles)
	fmt.Fprintf(&b, "| No-network profiles | %d |\n", report.Summary.NoNetworkProfiles)
	fmt.Fprintf(&b, "| Telemetry-disabled profiles | %d |\n", report.Summary.TelemetryDisabledProfiles)
	fmt.Fprintf(&b, "| Bundles | %d |\n", report.Summary.Bundles)
	fmt.Fprintf(&b, "| Pinned bundles | %d |\n", report.Summary.PinnedBundles)
	fmt.Fprintf(&b, "| Offline update bundles | %d |\n", report.Summary.OfflineUpdateBundles)
	fmt.Fprintf(&b, "| Rollback plans | %d |\n", report.Summary.RollbackPlans)
	fmt.Fprintf(&b, "| Reproducible commands | %d |\n", report.Summary.ReproducibleCommands)
	fmt.Fprintf(&b, "| Evidence artifacts | %d |\n", report.Summary.EvidenceArtifacts)
	fmt.Fprintf(&b, "| Counterexamples | %d |\n\n", report.Summary.Counterexamples)
	fmt.Fprintf(&b, "## Deployment profiles\n\n")
	fmt.Fprintf(&b, "| Profile | Environment | Target | Controls | No network | Telemetry off | Bundles | Updates | Rollback |\n| --- | --- | --- | ---: | ---: | ---: | ---: | ---: | ---: |\n")
	for _, profile := range report.Profiles {
		fmt.Fprintf(&b, "| `%s` | `%s` | `%s` | %d | `%t` | `%t` | %d | %d | `%t` |\n",
			escapeTable(profile.ID),
			escapeTable(profile.Environment),
			escapeTable(profile.InstallTarget),
			len(profile.RegulatoryControls),
			profile.NoNetwork,
			profile.TelemetryDisabled,
			len(profile.Bundles),
			len(profile.UpdateBundles),
			profile.Rollback.Present,
		)
	}
	if len(report.Counterexamples) > 0 {
		fmt.Fprintf(&b, "\n## Counterexamples\n\n")
		fmt.Fprintf(&b, "| ID | Kind | Subject | Message |\n| --- | --- | --- | --- |\n")
		for _, counterexample := range report.Counterexamples {
			fmt.Fprintf(&b, "| `%s` | `%s` | `%s` | %s |\n",
				escapeTable(counterexample.ID),
				escapeTable(counterexample.Kind),
				escapeTable(firstNonEmpty(counterexample.Subject, "-")),
				escapeTable(counterexample.Message),
			)
		}
	}
	return b.String()
}

func buildProfileReport(root string, profile Profile, criteria Criteria) (ProfileReport, []Counterexample) {
	subject := profile.ID
	if strings.TrimSpace(subject) == "" {
		subject = profile.Environment
	}
	networkOK, networkCounterexamples := validateNoNetwork(profile, criteria)
	telemetryOK, telemetryCounterexamples := validateTelemetryDisabled(profile, criteria)
	counterexamples := append(networkCounterexamples, telemetryCounterexamples...)
	evidence, evidenceCounterexamples := collectArtifacts(root, profile.EvidencePaths, subject, "evidence", "missing_evidence", "empty_evidence", "invalid_evidence_path", "deployment evidence could not be read")
	counterexamples = append(counterexamples, evidenceCounterexamples...)
	if criteria.RequireEvidenceHashes && len(evidence) == 0 {
		counterexamples = append(counterexamples, Counterexample{
			ID:      "profile." + stableID(subject, "evidence") + ".missing",
			Kind:    "missing_evidence",
			Subject: subject,
			Message: "profile does not cite readable deployment evidence to hash",
		})
	}

	regulatoryControls := sortedStrings(normalizedStrings(profile.RegulatoryControls))
	if len(regulatoryControls) < criteria.MinRegulatoryControlsPerProfile {
		counterexamples = append(counterexamples, Counterexample{
			ID:      "profile." + stableID(subject, "regulatory-controls") + ".insufficient",
			Kind:    "insufficient_regulatory_controls",
			Subject: subject,
			Message: fmt.Sprintf("profile has %d regulatory controls below required %d", len(regulatoryControls), criteria.MinRegulatoryControlsPerProfile),
		})
	}
	commands := profileCommands(profile)
	if criteria.RequireReproducibleCommands && len(commands) == 0 {
		counterexamples = append(counterexamples, Counterexample{
			ID:      "profile." + stableID(subject, "commands") + ".missing",
			Kind:    "missing_reproducible_command",
			Subject: subject,
			Message: "profile does not list local install, verification, or rollback commands",
		})
	}

	var bundles []BundleReport
	for _, bundle := range sortedBundles(profile.Bundles) {
		br, bundleCounterexamples := buildBundleReport(root, subject, bundle, criteria)
		bundles = append(bundles, br)
		counterexamples = append(counterexamples, bundleCounterexamples...)
	}
	if len(bundles) < criteria.MinBundlesPerProfile {
		counterexamples = append(counterexamples, Counterexample{
			ID:      "profile." + stableID(subject, "bundles") + ".insufficient",
			Kind:    "insufficient_bundles",
			Subject: subject,
			Message: fmt.Sprintf("profile has %d bundles below required %d", len(bundles), criteria.MinBundlesPerProfile),
		})
	}

	var updates []UpdateReport
	for _, update := range sortedUpdates(profile.UpdateBundles) {
		ur, updateCounterexamples := buildUpdateReport(root, subject, update, criteria)
		updates = append(updates, ur)
		counterexamples = append(counterexamples, updateCounterexamples...)
	}
	if len(updates) < criteria.MinUpdateBundlesPerProfile {
		counterexamples = append(counterexamples, Counterexample{
			ID:      "profile." + stableID(subject, "update-bundles") + ".insufficient",
			Kind:    "missing_update_bundle",
			Subject: subject,
			Message: fmt.Sprintf("profile has %d update bundles below required %d", len(updates), criteria.MinUpdateBundlesPerProfile),
		})
	}

	rollback, rollbackCounterexamples := buildRollbackReport(root, subject, profile.RollbackPlan, criteria)
	counterexamples = append(counterexamples, rollbackCounterexamples...)
	sortCounterexamples(counterexamples)
	return ProfileReport{
		ID:                   profile.ID,
		Environment:          normalizeToken(profile.Environment),
		Site:                 profile.Site,
		InstallTarget:        profile.InstallTarget,
		RegulatoryControls:   regulatoryControls,
		NoNetwork:            networkOK,
		TelemetryDisabled:    telemetryOK,
		ReproducibleCommands: len(commands),
		Evidence:             evidence,
		Bundles:              bundles,
		UpdateBundles:        updates,
		Rollback:             rollback,
	}, counterexamples
}

func buildBundleReport(root, subject string, bundle Bundle, criteria Criteria) (BundleReport, []Counterexample) {
	expected := normalizeHash(bundle.SHA256)
	artifact, artifactCounterexamples := collectArtifact(root, bundle.Path, subject+"/"+bundle.ID, "bundle", "missing_bundle", "empty_bundle", "invalid_bundle_path", "offline deployment bundle could not be read")
	counterexamples := artifactCounterexamples
	if criteria.RequirePinnedBundles && expected == "" {
		counterexamples = append(counterexamples, Counterexample{
			ID:      "bundle." + stableID(subject, bundle.ID, "sha256") + ".missing",
			Kind:    "unpinned_bundle",
			Subject: subject + "/" + bundle.ID,
			Message: "bundle does not declare a sha256 pin",
			Witness: []string{bundle.Path},
		})
	}
	if expected != "" && artifact.Path != "" && artifact.SHA256 != expected {
		counterexamples = append(counterexamples, Counterexample{
			ID:      "bundle." + stableID(subject, bundle.ID, "sha256") + ".mismatch",
			Kind:    "bundle_hash_mismatch",
			Subject: subject + "/" + bundle.ID,
			Message: "bundle sha256 pin does not match artifact bytes",
			Witness: []string{bundle.Path, expected, artifact.SHA256},
		})
	}
	signature, signatureCounterexamples := collectOptionalArtifact(root, bundle.SignaturePath, subject+"/"+bundle.ID, "signature", criteria.RequireBundleSignatures, "missing_signature")
	counterexamples = append(counterexamples, signatureCounterexamples...)
	sbom, sbomCounterexamples := collectOptionalArtifact(root, bundle.SBOMPath, subject+"/"+bundle.ID, "sbom", criteria.RequireSoftwareBillOfMaterials, "missing_sbom")
	counterexamples = append(counterexamples, sbomCounterexamples...)
	pinned := expected != "" && artifact.Path != "" && artifact.SHA256 == expected
	return BundleReport{
		ID:        bundle.ID,
		Kind:      normalizeToken(bundle.Kind),
		Version:   bundle.Version,
		Channel:   normalizeToken(bundle.UpdateChannel),
		Artifact:  artifact,
		Expected:  expected,
		Pinned:    pinned,
		Signature: signature,
		SBOM:      sbom,
	}, counterexamples
}

func buildUpdateReport(root, subject string, update UpdateBundle, criteria Criteria) (UpdateReport, []Counterexample) {
	expected := normalizeHash(update.SHA256)
	artifact, artifactCounterexamples := collectArtifact(root, update.Path, subject+"/"+update.ID, "update", "missing_update_bundle", "empty_update_bundle", "invalid_update_path", "offline update bundle could not be read")
	counterexamples := artifactCounterexamples
	if criteria.RequirePinnedUpdateBundles && expected == "" {
		counterexamples = append(counterexamples, Counterexample{
			ID:      "update." + stableID(subject, update.ID, "sha256") + ".missing",
			Kind:    "unpinned_update_bundle",
			Subject: subject + "/" + update.ID,
			Message: "update bundle does not declare a sha256 pin",
			Witness: []string{update.Path},
		})
	}
	if expected != "" && artifact.Path != "" && artifact.SHA256 != expected {
		counterexamples = append(counterexamples, Counterexample{
			ID:      "update." + stableID(subject, update.ID, "sha256") + ".mismatch",
			Kind:    "update_hash_mismatch",
			Subject: subject + "/" + update.ID,
			Message: "update bundle sha256 pin does not match artifact bytes",
			Witness: []string{update.Path, expected, artifact.SHA256},
		})
	}
	if criteria.RequireOfflineUpdates && !update.Offline {
		counterexamples = append(counterexamples, Counterexample{
			ID:      "update." + stableID(subject, update.ID, "offline") + ".false",
			Kind:    "update_not_offline",
			Subject: subject + "/" + update.ID,
			Message: "update bundle is not marked as applicable offline",
			Witness: []string{update.Path},
		})
	}
	manifest, manifestCounterexamples := collectOptionalArtifact(root, update.ManifestPath, subject+"/"+update.ID, "manifest", criteria.RequirePinnedUpdateBundles, "missing_update_manifest")
	counterexamples = append(counterexamples, manifestCounterexamples...)
	signature, signatureCounterexamples := collectOptionalArtifact(root, update.SignaturePath, subject+"/"+update.ID, "update-signature", criteria.RequireBundleSignatures, "missing_signature")
	counterexamples = append(counterexamples, signatureCounterexamples...)
	return UpdateReport{
		ID:        update.ID,
		From:      update.FromVersion,
		To:        update.ToVersion,
		Offline:   update.Offline,
		AppliesTo: sortedStrings(normalizedStrings(update.AppliesTo)),
		Artifact:  artifact,
		Expected:  expected,
		Manifest:  manifest,
		Signature: signature,
	}, counterexamples
}

func buildRollbackReport(root, subject string, rollback RollbackPlan, criteria Criteria) (RollbackReport, []Counterexample) {
	present := strings.TrimSpace(rollback.ID) != "" && rollback.MaxMinutes > 0 && normalizeHash(rollback.PreviousBundleSHA256) != "" && len(rollback.Commands) > 0
	evidence, evidenceCounterexamples := collectArtifacts(root, rollback.EvidencePaths, subject+"/rollback", "rollback-evidence", "missing_rollback_evidence", "empty_rollback_evidence", "invalid_rollback_evidence_path", "rollback evidence could not be read")
	counterexamples := evidenceCounterexamples
	if criteria.RequireRollbackPlan {
		if !present {
			counterexamples = append(counterexamples, Counterexample{
				ID:      "rollback." + stableID(subject, "plan") + ".missing",
				Kind:    "missing_rollback_plan",
				Subject: subject,
				Message: "profile lacks an id, time bound, previous bundle pin, and rollback commands",
			})
		}
		if len(evidence) == 0 {
			counterexamples = append(counterexamples, Counterexample{
				ID:      "rollback." + stableID(subject, "evidence") + ".missing",
				Kind:    "missing_rollback_evidence",
				Subject: subject,
				Message: "rollback plan does not cite readable rollback evidence",
			})
		}
	}
	return RollbackReport{
		ID:                   rollback.ID,
		Present:              present,
		MaxMinutes:           rollback.MaxMinutes,
		PreviousBundleSHA256: normalizeHash(rollback.PreviousBundleSHA256),
		Commands:             len(rollback.Commands),
		Evidence:             evidence,
	}, counterexamples
}

func validateNoNetwork(profile Profile, criteria Criteria) (bool, []Counterexample) {
	var counterexamples []Counterexample
	mode := normalizeToken(profile.NetworkPolicy.Mode)
	noNetwork := inSet(mode, "none", "off", "offline", "airgapped", "air-gapped", "disabled") && !profile.NetworkPolicy.EgressAllowed && len(normalizedStrings(profile.NetworkPolicy.AllowedEndpoints)) == 0
	if criteria.RequireNoNetwork && !noNetwork {
		counterexamples = append(counterexamples, Counterexample{
			ID:      "profile." + stableID(profile.ID, "network-policy") + ".egress",
			Kind:    "network_not_disabled",
			Subject: profile.ID,
			Message: "network policy must disable all egress and allowed endpoints",
			Witness: append([]string{profile.NetworkPolicy.Mode}, profile.NetworkPolicy.AllowedEndpoints...),
		})
	}
	for _, command := range profileCommands(profile) {
		if reason := networkCommandReason(command); reason != "" {
			noNetwork = false
			counterexamples = append(counterexamples, Counterexample{
				ID:      "command." + stableID(profile.ID, command, "network") + ".forbidden",
				Kind:    "network_command",
				Subject: profile.ID,
				Message: "offline deployment command appears to require network access: " + reason,
				Witness: []string{command},
			})
		}
	}
	return noNetwork, counterexamples
}

func validateTelemetryDisabled(profile Profile, criteria Criteria) (bool, []Counterexample) {
	var counterexamples []Counterexample
	mode := normalizeToken(profile.TelemetryPolicy.Mode)
	disabled := inSet(mode, "none", "off", "disabled") && !profile.TelemetryPolicy.Enabled && len(normalizedStrings(profile.TelemetryPolicy.Destinations)) == 0
	if criteria.RequireTelemetryDisabled && !disabled {
		counterexamples = append(counterexamples, Counterexample{
			ID:      "profile." + stableID(profile.ID, "telemetry-policy") + ".enabled",
			Kind:    "telemetry_enabled",
			Subject: profile.ID,
			Message: "telemetry policy must be disabled with no destinations",
			Witness: append([]string{profile.TelemetryPolicy.Mode}, profile.TelemetryPolicy.Destinations...),
		})
	}
	for _, command := range profileCommands(profile) {
		if reason := telemetryCommandReason(command); reason != "" {
			disabled = false
			counterexamples = append(counterexamples, Counterexample{
				ID:      "command." + stableID(profile.ID, command, "telemetry") + ".forbidden",
				Kind:    "telemetry_command",
				Subject: profile.ID,
				Message: "offline deployment command appears to enable telemetry: " + reason,
				Witness: []string{command},
			})
		}
	}
	return disabled, counterexamples
}

func collectOptionalArtifact(root, relPath, subject, idPart string, required bool, missingKind string) (ArtifactEvidence, []Counterexample) {
	if strings.TrimSpace(relPath) == "" {
		if !required {
			return ArtifactEvidence{}, nil
		}
		return ArtifactEvidence{}, []Counterexample{{
			ID:      "artifact." + stableID(subject, idPart) + ".missing",
			Kind:    missingKind,
			Subject: subject,
			Message: idPart + " artifact path is required",
		}}
	}
	artifact, counterexamples := collectArtifact(root, relPath, subject, idPart, missingKind, "empty_"+idPart, "invalid_"+idPart+"_path", idPart+" artifact could not be read")
	return artifact, counterexamples
}

func collectArtifacts(root string, paths []string, subject, idPart, missingKind, emptyKind, invalidKind, missingMessage string) ([]ArtifactEvidence, []Counterexample) {
	var artifacts []ArtifactEvidence
	var counterexamples []Counterexample
	for _, path := range uniqueSorted(paths) {
		artifact, artifactCounterexamples := collectArtifact(root, path, subject, idPart, missingKind, emptyKind, invalidKind, missingMessage)
		counterexamples = append(counterexamples, artifactCounterexamples...)
		if artifact.Path != "" {
			artifacts = append(artifacts, artifact)
		}
	}
	return artifacts, counterexamples
}

func collectArtifact(root, relPath, subject, idPart, missingKind, emptyKind, invalidKind, missingMessage string) (ArtifactEvidence, []Counterexample) {
	fullPath, err := safeJoin(root, relPath)
	if err != nil {
		return ArtifactEvidence{}, []Counterexample{{
			ID:      "artifact." + stableID(subject, relPath, idPart, "path") + ".invalid",
			Kind:    invalidKind,
			Subject: subject,
			Message: err.Error(),
			Witness: []string{relPath},
		}}
	}
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return ArtifactEvidence{}, []Counterexample{{
			ID:      "artifact." + stableID(subject, relPath, idPart) + ".missing",
			Kind:    missingKind,
			Subject: subject,
			Message: missingMessage,
			Witness: []string{relPath},
		}}
	}
	if len(data) == 0 {
		return ArtifactEvidence{}, []Counterexample{{
			ID:      "artifact." + stableID(subject, relPath, idPart) + ".empty",
			Kind:    emptyKind,
			Subject: subject,
			Message: "offline deployment artifact is empty",
			Witness: []string{relPath},
		}}
	}
	sum := sha256.Sum256(data)
	return ArtifactEvidence{
		Path:   filepath.ToSlash(filepath.Clean(relPath)),
		SHA256: "sha256:" + hex.EncodeToString(sum[:]),
		Bytes:  int64(len(data)),
	}, nil
}

func validateSpec(spec Spec) error {
	if strings.TrimSpace(spec.Name) == "" {
		return fmt.Errorf("offline deploy name is required")
	}
	criteria := spec.Criteria
	if len(criteria.RequiredEnvironments) == 0 {
		return fmt.Errorf("criteria.required_environments is required")
	}
	if criteria.MinProfiles <= 0 || criteria.MinBundlesPerProfile <= 0 || criteria.MinUpdateBundlesPerProfile <= 0 || criteria.MinRegulatoryControlsPerProfile <= 0 {
		return fmt.Errorf("offline deploy minimum criteria must be positive")
	}
	profileIDs := map[string]struct{}{}
	for _, profile := range spec.Profiles {
		if strings.TrimSpace(profile.ID) == "" {
			return fmt.Errorf("profile id is required")
		}
		if _, ok := profileIDs[profile.ID]; ok {
			return fmt.Errorf("duplicate profile id %q", profile.ID)
		}
		profileIDs[profile.ID] = struct{}{}
		if strings.TrimSpace(profile.Environment) == "" || strings.TrimSpace(profile.InstallTarget) == "" {
			return fmt.Errorf("profile %q requires environment and install_target", profile.ID)
		}
		for _, path := range profile.EvidencePaths {
			if err := validateRelativePath(path); err != nil {
				return fmt.Errorf("profile %q evidence path: %w", profile.ID, err)
			}
		}
		bundleIDs := map[string]struct{}{}
		for _, bundle := range profile.Bundles {
			if strings.TrimSpace(bundle.ID) == "" {
				return fmt.Errorf("profile %q bundle id is required", profile.ID)
			}
			if _, ok := bundleIDs[bundle.ID]; ok {
				return fmt.Errorf("profile %q contains duplicate bundle id %q", profile.ID, bundle.ID)
			}
			bundleIDs[bundle.ID] = struct{}{}
			if strings.TrimSpace(bundle.Path) == "" || strings.TrimSpace(bundle.Version) == "" {
				return fmt.Errorf("profile %q bundle %q requires path and version", profile.ID, bundle.ID)
			}
			for _, path := range []string{bundle.Path, bundle.SignaturePath, bundle.SBOMPath} {
				if strings.TrimSpace(path) == "" {
					continue
				}
				if err := validateRelativePath(path); err != nil {
					return fmt.Errorf("profile %q bundle %q path: %w", profile.ID, bundle.ID, err)
				}
			}
			if bundle.SHA256 != "" && normalizeHash(bundle.SHA256) == "" {
				return fmt.Errorf("profile %q bundle %q sha256 must be sha256-prefixed or 64 hex characters", profile.ID, bundle.ID)
			}
		}
		updateIDs := map[string]struct{}{}
		for _, update := range profile.UpdateBundles {
			if strings.TrimSpace(update.ID) == "" {
				return fmt.Errorf("profile %q update bundle id is required", profile.ID)
			}
			if _, ok := updateIDs[update.ID]; ok {
				return fmt.Errorf("profile %q contains duplicate update bundle id %q", profile.ID, update.ID)
			}
			updateIDs[update.ID] = struct{}{}
			if strings.TrimSpace(update.Path) == "" || strings.TrimSpace(update.FromVersion) == "" || strings.TrimSpace(update.ToVersion) == "" {
				return fmt.Errorf("profile %q update bundle %q requires path, from_version, and to_version", profile.ID, update.ID)
			}
			for _, path := range []string{update.Path, update.ManifestPath, update.SignaturePath} {
				if strings.TrimSpace(path) == "" {
					continue
				}
				if err := validateRelativePath(path); err != nil {
					return fmt.Errorf("profile %q update bundle %q path: %w", profile.ID, update.ID, err)
				}
			}
			if update.SHA256 != "" && normalizeHash(update.SHA256) == "" {
				return fmt.Errorf("profile %q update bundle %q sha256 must be sha256-prefixed or 64 hex characters", profile.ID, update.ID)
			}
		}
		for _, path := range profile.RollbackPlan.EvidencePaths {
			if err := validateRelativePath(path); err != nil {
				return fmt.Errorf("profile %q rollback evidence path: %w", profile.ID, err)
			}
		}
		if profile.RollbackPlan.PreviousBundleSHA256 != "" && normalizeHash(profile.RollbackPlan.PreviousBundleSHA256) == "" {
			return fmt.Errorf("profile %q rollback previous bundle sha256 must be sha256-prefixed or 64 hex characters", profile.ID)
		}
	}
	return nil
}

func profileCommands(profile Profile) []string {
	commands := append([]string{}, profile.InstallCommands...)
	commands = append(commands, profile.VerifyCommands...)
	commands = append(commands, profile.RollbackPlan.Commands...)
	return trimmedStrings(commands)
}

var (
	urlPattern     = regexp.MustCompile(`(?i)\b[a-z][a-z0-9+.-]*://`)
	domainPattern  = regexp.MustCompile(`(?i)\b[a-z0-9][a-z0-9.-]+\.(com|net|org|io|dev|cloud|app|gov|edu)(:[0-9]+)?\b`)
	ipv4Pattern    = regexp.MustCompile(`\b([0-9]{1,3}\.){3}[0-9]{1,3}\b`)
	networkToolSet = map[string]struct{}{
		"curl": {}, "wget": {}, "ftp": {}, "sftp": {}, "ssh": {}, "scp": {}, "rsync": {},
		"nc": {}, "netcat": {}, "telnet": {}, "gh": {}, "aws": {}, "az": {}, "gcloud": {},
	}
	networkPhrases = []string{
		"git clone", "git fetch", "git pull", "docker pull", "podman pull", "kubectl apply",
		"apt-get update", "apt update", "brew install", "pip install ", "npm install ",
		"go get ", "go install ",
	}
	telemetryPhrases = []string{
		"datadog", "sentry", "segment", "opentelemetry", "otel-collector", "honeycomb", "newrelic",
	}
)

func networkCommandReason(command string) string {
	lower := strings.ToLower(command)
	if urlPattern.MatchString(lower) {
		return "url"
	}
	if domainPattern.MatchString(lower) {
		return "host"
	}
	if ipv4Pattern.MatchString(lower) {
		return "ip-address"
	}
	for _, phrase := range networkPhrases {
		if strings.Contains(lower, phrase) {
			if strings.HasPrefix(phrase, "pip install") && strings.Contains(lower, "--no-index") {
				continue
			}
			if strings.HasPrefix(phrase, "npm install") && strings.Contains(lower, "--offline") {
				continue
			}
			return strings.TrimSpace(phrase)
		}
	}
	for _, token := range commandTokens(lower) {
		if _, ok := networkToolSet[token]; ok {
			return token
		}
	}
	return ""
}

func telemetryCommandReason(command string) string {
	lower := strings.ToLower(command)
	if strings.Contains(lower, "telemetry=off") || strings.Contains(lower, "telemetry=disabled") || strings.Contains(lower, "telemetry off") {
		return ""
	}
	if strings.Contains(lower, "telemetry") {
		return "telemetry"
	}
	for _, phrase := range telemetryPhrases {
		if strings.Contains(lower, phrase) {
			return phrase
		}
	}
	return ""
}

func commandTokens(command string) []string {
	return strings.FieldsFunc(command, func(r rune) bool {
		return !(unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_' || r == '.')
	})
}

func normalizeCriteria(criteria Criteria) Criteria {
	criteria.RequiredEnvironments = sortedStrings(normalizedStrings(criteria.RequiredEnvironments))
	return criteria
}

func sortedProfiles(profiles []Profile) []Profile {
	out := append([]Profile(nil), profiles...)
	sort.SliceStable(out, func(i, j int) bool { return normalizeToken(out[i].ID) < normalizeToken(out[j].ID) })
	return out
}

func sortedBundles(bundles []Bundle) []Bundle {
	out := append([]Bundle(nil), bundles...)
	sort.SliceStable(out, func(i, j int) bool { return normalizeToken(out[i].ID) < normalizeToken(out[j].ID) })
	return out
}

func sortedUpdates(updates []UpdateBundle) []UpdateBundle {
	out := append([]UpdateBundle(nil), updates...)
	sort.SliceStable(out, func(i, j int) bool { return normalizeToken(out[i].ID) < normalizeToken(out[j].ID) })
	return out
}

func sortCounterexamples(counterexamples []Counterexample) {
	sort.SliceStable(counterexamples, func(i, j int) bool { return counterexamples[i].ID < counterexamples[j].ID })
}

func sortedStrings(values []string) []string {
	out := append([]string(nil), values...)
	sort.Strings(out)
	return out
}

func normalizedStrings(values []string) []string {
	var out []string
	seen := map[string]struct{}{}
	for _, value := range values {
		normalized := normalizeToken(value)
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}
	sort.Strings(out)
	return out
}

func trimmedStrings(values []string) []string {
	var out []string
	seen := map[string]struct{}{}
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	sort.Strings(out)
	return out
}

func uniqueSorted(values []string) []string {
	var out []string
	seen := map[string]struct{}{}
	for _, value := range values {
		clean := filepath.ToSlash(filepath.Clean(strings.TrimSpace(value)))
		if clean == "." || clean == "" {
			continue
		}
		if _, ok := seen[clean]; ok {
			continue
		}
		seen[clean] = struct{}{}
		out = append(out, clean)
	}
	sort.Strings(out)
	return out
}

func normalizeToken(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	lastDash := false
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

func normalizeHash(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.TrimPrefix(value, "sha256:")
	if len(value) != 64 {
		return ""
	}
	for _, r := range value {
		if !((r >= 'a' && r <= 'f') || (r >= '0' && r <= '9')) {
			return ""
		}
	}
	return "sha256:" + value
}

func safeJoin(root, relPath string) (string, error) {
	if err := validateRelativePath(relPath); err != nil {
		return "", err
	}
	clean := filepath.Clean(filepath.FromSlash(relPath))
	fullPath := filepath.Join(root, clean)
	rel, err := filepath.Rel(root, fullPath)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path %q escapes root", relPath)
	}
	return fullPath, nil
}

func validateRelativePath(path string) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("path is required")
	}
	if filepath.IsAbs(path) {
		return fmt.Errorf("path %q must be relative", path)
	}
	clean := filepath.Clean(filepath.FromSlash(path))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return fmt.Errorf("path %q escapes root", path)
	}
	return nil
}

func inSet(value string, allowed ...string) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}

func stableID(parts ...string) string {
	joined := strings.Join(parts, "\x00")
	sum := sha256.Sum256([]byte(joined))
	return hex.EncodeToString(sum[:])[:16]
}

func reportHash(report Report) string {
	report.Hash = ""
	return canonical.Hash(report)
}

func escapeTable(value string) string {
	return strings.ReplaceAll(value, "|", "\\|")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
