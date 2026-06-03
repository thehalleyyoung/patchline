package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/thehalleyyoung/patchline/internal/evidencemarketplace"
)

func evidenceMarketplaceCommand(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: patchline evidence-marketplace <publish|challenge> --registry registry.json --out dir [--json]")
	}
	switch args[0] {
	case "publish":
		return evidenceMarketplacePublish(args[1:], hasFlag(args[1:], "--json"))
	case "challenge":
		return evidenceMarketplaceChallenge(args[1:], hasFlag(args[1:], "--json"))
	default:
		return fmt.Errorf("unknown evidence-marketplace command %q", args[0])
	}
}

func evidenceMarketplacePublish(args []string, jsonOut bool) error {
	registryPath, ok := flagValue(args, "--registry")
	if !ok || registryPath == "" {
		return errors.New("usage: patchline evidence-marketplace publish --registry registry.json --out dir [--json]")
	}
	outPath, ok := flagValue(args, "--out")
	if !ok || outPath == "" {
		return errors.New("usage: patchline evidence-marketplace publish --registry registry.json --out dir [--json]")
	}
	report, err := evidencemarketplace.PublishRegistryFile(registryPath)
	if err != nil {
		return err
	}
	if err := evidencemarketplace.WriteReport(outPath, report); err != nil {
		return err
	}
	if jsonOut {
		if err := writeJSON(os.Stdout, report); err != nil {
			return err
		}
	}
	if !report.OK {
		return codedError{code: 2, err: fmt.Errorf("evidence marketplace rejected %d submitted example(s)", report.Summary.Rejected)}
	}
	if !jsonOut {
		fmt.Printf("published %d redacted certificate-backed evidence example(s) to %s\n", report.Summary.Published, outPath)
	}
	return nil
}

func evidenceMarketplaceChallenge(args []string, jsonOut bool) error {
	registryPath, ok := flagValue(args, "--registry")
	if !ok || registryPath == "" {
		return errors.New("usage: patchline evidence-marketplace challenge --registry registry.json --out dir [--json]")
	}
	outPath, ok := flagValue(args, "--out")
	if !ok || outPath == "" {
		return errors.New("usage: patchline evidence-marketplace challenge --registry registry.json --out dir [--json]")
	}
	report, err := evidencemarketplace.PublishChallengeTrackFile(registryPath)
	if err != nil {
		return err
	}
	if err := evidencemarketplace.WriteChallengeReport(outPath, report); err != nil {
		return err
	}
	if jsonOut {
		if err := writeJSON(os.Stdout, report); err != nil {
			return err
		}
	}
	if !report.OK {
		return codedError{code: 2, err: fmt.Errorf("adversarial challenge rejected %d submitted example(s)", report.Summary.Rejected)}
	}
	if !jsonOut {
		fmt.Printf("scored %d adversarial migration challenge submission(s); %d reached the scoreboard in %s\n", report.Summary.Accepted, report.Summary.ScoreboardEntries, outPath)
	}
	return nil
}
