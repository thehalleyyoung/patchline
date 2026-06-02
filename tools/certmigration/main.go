package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/thehalleyyoung/patchline/internal/certlang"
)

func main() {
	specDir := flag.String("spec-dir", "specs/certificate-interchange/v0", "legacy PLCI spec directory")
	root := flag.String("root", ".", "repository root for file: evidence digests")
	jsonOut := flag.Bool("json", false, "write JSON report")
	flag.Parse()

	report, err := certlang.CheckMigrationDirectory(*specDir, certlang.Options{Root: *root, VerifyFiles: true})
	if *jsonOut {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if encodeErr := encoder.Encode(report); encodeErr != nil {
			fmt.Fprintln(os.Stderr, encodeErr)
			os.Exit(1)
		}
	} else if err == nil {
		fmt.Printf("PLCI migration: source=%s target=%s migrated=%d rejected=%d all_ok=%t\n",
			report.SourceVersion, report.TargetVersion, report.Migrated, report.Rejected, report.AllOK)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if !report.AllOK {
		os.Exit(1)
	}
}
