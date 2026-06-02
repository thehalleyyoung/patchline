package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/thehalleyyoung/patchline/internal/certlang"
)

func main() {
	specDir := flag.String("spec-dir", "specs/certificate-interchange/v1", "PLCI/1 spec directory")
	root := flag.String("root", ".", "repository root for file: evidence digests")
	jsonOut := flag.Bool("json", false, "write JSON report")
	flag.Parse()

	report, err := certlang.CheckDirectory(*specDir, certlang.Options{Root: *root, VerifyFiles: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if *jsonOut {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(report); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	} else {
		fmt.Printf("go PLCI/1 checker: valid=%d invalid=%d all_ok=%t\n", report.TotalValid, report.TotalInvalid, report.AllOK)
	}
	if !report.AllOK {
		os.Exit(1)
	}
}
