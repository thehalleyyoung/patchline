package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/thehalleyyoung/patchline/internal/verdictx"
)

func main() {
	specDir := flag.String("spec-dir", "specs/verdict-exchange/v1", "verdict exchange spec directory")
	root := flag.String("root", ".", "repository root for file digest verification")
	jsonOut := flag.Bool("json", false, "write JSON report")
	flag.Parse()

	report, err := verdictx.RunSuite(*specDir, *root)
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
		fmt.Printf("verdict exchange: analyzers=%d cases=%d roundtrips=%d negative_controls=%d verified=%t\n",
			len(report.Analyzers),
			report.PositiveCases,
			report.RoundTrips,
			report.NegativeControlsPassed,
			report.Verified,
		)
	}
	if !report.Verified {
		os.Exit(1)
	}
}
