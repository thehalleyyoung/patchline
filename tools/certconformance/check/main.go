package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/thehalleyyoung/patchline/internal/certconformance"
)

func main() {
	corpus := flag.String("corpus", "specs/certificate-conformance/v1/corpus.json", "standards-body conformance corpus")
	root := flag.String("root", ".", "repository root for file evidence")
	jsonOut := flag.Bool("json", false, "write JSON report")
	flag.Parse()

	report, err := certconformance.Verify(*corpus, *root)
	if *jsonOut {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if encodeErr := encoder.Encode(report); encodeErr != nil {
			fmt.Fprintln(os.Stderr, encodeErr)
			os.Exit(1)
		}
	} else if err == nil {
		fmt.Printf("certificate conformance corpus: cases=%d all_ok=%t\n", report.TotalCases, report.AllOK)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
