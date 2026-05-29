package invariant

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"

	"github.com/thehalleyyoung/patchline/internal/canonical"
	"github.com/thehalleyyoung/patchline/internal/replay"
)

const Version = "patchline.invariants/v1"

type Spec struct {
	Version    string        `json:"version"`
	Name       string        `json:"name"`
	Invariants []Declaration `json:"invariants"`
}

type Declaration struct {
	ID          string   `json:"id"`
	Kind        string   `json:"kind"`
	Table       string   `json:"table"`
	Column      string   `json:"column,omitempty"`
	RefTable    string   `json:"ref_table,omitempty"`
	RefColumn   string   `json:"ref_column,omitempty"`
	GroupColumn string   `json:"group_column,omitempty"`
	Values      []string `json:"values,omitempty"`
	Expect      string   `json:"expect,omitempty"`
}

type Report struct {
	Version    string        `json:"version"`
	Name       string        `json:"name"`
	OK         bool          `json:"ok"`
	Checks     []Check       `json:"checks"`
	Candidates []Declaration `json:"candidates,omitempty"`
	Hash       string        `json:"hash"`
}

type Check struct {
	ID              string           `json:"id"`
	Kind            string           `json:"kind"`
	Table           string           `json:"table"`
	Column          string           `json:"column,omitempty"`
	Status          string           `json:"status"`
	Support         int              `json:"support"`
	Counterexamples []Counterexample `json:"counterexamples,omitempty"`
	Reason          string           `json:"reason"`
}

type Counterexample struct {
	Table   string `json:"table"`
	RowID   string `json:"row_id,omitempty"`
	Column  string `json:"column,omitempty"`
	Value   string `json:"value,omitempty"`
	Message string `json:"message"`
}

func Read(r io.Reader) (Spec, error) {
	decoder := json.NewDecoder(r)
	decoder.DisallowUnknownFields()
	var spec Spec
	if err := decoder.Decode(&spec); err != nil {
		return Spec{}, err
	}
	if spec.Version != Version {
		return Spec{}, fmt.Errorf("invariant spec version must be %s", Version)
	}
	return spec, nil
}

func CheckStore(spec Spec, store replay.Store) Report {
	report := Report{
		Version: Version,
		Name:    spec.Name,
		OK:      true,
	}
	declarations := append([]Declaration(nil), spec.Invariants...)
	sort.Slice(declarations, func(i, j int) bool {
		return declarations[i].ID < declarations[j].ID
	})
	for _, declaration := range declarations {
		check := checkDeclaration(declaration, store)
		if check.Status != "checked" {
			report.OK = false
		}
		report.Checks = append(report.Checks, check)
	}
	report.Hash = reportHash(report)
	return report
}

func Discover(store replay.Store) Report {
	report := Report{
		Version: Version,
		Name:    "candidate-discovery",
		OK:      true,
	}
	for _, tableName := range tableNames(store) {
		rows := store.Tables[tableName]
		if len(rows) == 0 {
			continue
		}
		columns := columns(rows)
		for _, column := range columns {
			values := columnValues(rows, column)
			if column == "id" && allUnique(values) {
				report.Candidates = append(report.Candidates, Declaration{
					ID:     "candidate." + tableName + "." + column + ".unique",
					Kind:   "unique",
					Table:  tableName,
					Column: column,
				})
			}
			if allNonNegativeInts(values) {
				report.Candidates = append(report.Candidates, Declaration{
					ID:     "candidate." + tableName + "." + column + ".nonnegative",
					Kind:   "nonnegative",
					Table:  tableName,
					Column: column,
				})
			}
			distinctValues := distinct(values)
			if len(distinctValues) > 0 && len(distinctValues) <= 5 {
				report.Candidates = append(report.Candidates, Declaration{
					ID:     "candidate." + tableName + "." + column + ".enum",
					Kind:   "enum",
					Table:  tableName,
					Column: column,
					Values: distinctValues,
				})
			}
		}
		report.Candidates = append(report.Candidates, Declaration{
			ID:     "candidate." + tableName + ".count",
			Kind:   "count",
			Table:  tableName,
			Expect: strconv.Itoa(len(rows)),
		})
	}
	sort.Slice(report.Candidates, func(i, j int) bool {
		return report.Candidates[i].ID < report.Candidates[j].ID
	})
	report.Hash = reportHash(report)
	return report
}

func checkDeclaration(declaration Declaration, store replay.Store) Check {
	check := Check{
		ID:     declaration.ID,
		Kind:   declaration.Kind,
		Table:  declaration.Table,
		Column: declaration.Column,
		Status: "checked",
	}
	rows, ok := store.Tables[declaration.Table]
	if !ok {
		check.Status = "refuted"
		check.Reason = "table is missing from replay store"
		check.Counterexamples = append(check.Counterexamples, Counterexample{
			Table:   declaration.Table,
			Message: "table not found",
		})
		return check
	}
	switch declaration.Kind {
	case "unique":
		checkUnique(declaration, rows, &check)
	case "enum":
		checkEnum(declaration, rows, &check)
	case "nonnegative":
		checkNonNegative(declaration, rows, &check)
	case "count":
		checkCount(declaration, rows, &check)
	case "foreign_key":
		checkForeignKey(declaration, rows, store, &check)
	case "sum":
		checkSum(declaration, rows, &check)
	case "materialized_report":
		checkMaterializedReport(declaration, rows, store, &check)
	case "ledger_balance":
		checkLedgerBalance(declaration, rows, &check)
	case "customer_total":
		checkCustomerTotal(declaration, rows, &check)
	default:
		check.Status = "unsupported"
		check.Reason = "unsupported invariant kind"
	}
	if len(check.Counterexamples) > 0 {
		check.Status = "refuted"
	}
	return check
}

func checkUnique(declaration Declaration, rows map[string]replay.Row, check *Check) {
	seen := map[string]string{}
	for _, rowID := range sortedRowIDs(rows) {
		value := rows[rowID][declaration.Column]
		if value == "" {
			continue
		}
		check.Support++
		if prior, ok := seen[value]; ok {
			check.Counterexamples = append(check.Counterexamples, Counterexample{
				Table: declaration.Table, RowID: rowID, Column: declaration.Column, Value: value,
				Message: "duplicate value also appears in row " + prior,
			})
			continue
		}
		seen[value] = rowID
	}
	check.Reason = "column values are unique over checked rows"
}

func checkEnum(declaration Declaration, rows map[string]replay.Row, check *Check) {
	allowed := map[string]struct{}{}
	for _, value := range declaration.Values {
		allowed[value] = struct{}{}
	}
	for _, rowID := range sortedRowIDs(rows) {
		value := rows[rowID][declaration.Column]
		if value == "" {
			continue
		}
		check.Support++
		if _, ok := allowed[value]; !ok {
			check.Counterexamples = append(check.Counterexamples, Counterexample{
				Table: declaration.Table, RowID: rowID, Column: declaration.Column, Value: value,
				Message: "value is outside declared enum",
			})
		}
	}
	check.Reason = "column values are inside declared enum"
}

func checkNonNegative(declaration Declaration, rows map[string]replay.Row, check *Check) {
	for _, rowID := range sortedRowIDs(rows) {
		value := rows[rowID][declaration.Column]
		if value == "" {
			continue
		}
		check.Support++
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 0 {
			check.Counterexamples = append(check.Counterexamples, Counterexample{
				Table: declaration.Table, RowID: rowID, Column: declaration.Column, Value: value,
				Message: "value is not a nonnegative integer",
			})
		}
	}
	check.Reason = "column values are nonnegative integers"
}

func checkCount(declaration Declaration, rows map[string]replay.Row, check *Check) {
	check.Support = len(rows)
	want, err := strconv.Atoi(declaration.Expect)
	if err != nil {
		check.Status = "refuted"
		check.Reason = "count invariant expect value must be an integer"
		return
	}
	if len(rows) != want {
		check.Counterexamples = append(check.Counterexamples, Counterexample{
			Table: declaration.Table, Value: strconv.Itoa(len(rows)),
			Message: fmt.Sprintf("row count differs from expected %d", want),
		})
	}
	check.Reason = "table row count matches expected value"
}

func checkForeignKey(declaration Declaration, rows map[string]replay.Row, store replay.Store, check *Check) {
	refRows, ok := store.Tables[declaration.RefTable]
	if !ok {
		check.Counterexamples = append(check.Counterexamples, Counterexample{Table: declaration.RefTable, Message: "referenced table not found"})
		return
	}
	refValues := map[string]struct{}{}
	for _, row := range refRows {
		if value := row[declaration.RefColumn]; value != "" {
			refValues[value] = struct{}{}
		}
	}
	for _, rowID := range sortedRowIDs(rows) {
		value := rows[rowID][declaration.Column]
		if value == "" {
			continue
		}
		check.Support++
		if _, ok := refValues[value]; !ok {
			check.Counterexamples = append(check.Counterexamples, Counterexample{
				Table: declaration.Table, RowID: rowID, Column: declaration.Column, Value: value,
				Message: "foreign key value has no referenced row",
			})
		}
	}
	check.Reason = "foreign key values exist in referenced table"
}

func checkSum(declaration Declaration, rows map[string]replay.Row, check *Check) {
	sum, ok := sumColumn(declaration, rows, check)
	if !ok {
		return
	}
	want, err := strconv.Atoi(declaration.Expect)
	if err != nil {
		check.Counterexamples = append(check.Counterexamples, Counterexample{Table: declaration.Table, Value: declaration.Expect, Message: "sum expect value must be an integer"})
		return
	}
	if sum != want {
		check.Counterexamples = append(check.Counterexamples, Counterexample{Table: declaration.Table, Column: declaration.Column, Value: strconv.Itoa(sum), Message: fmt.Sprintf("sum differs from expected %d", want)})
	}
	check.Reason = "column sum matches expected value"
}

func checkMaterializedReport(declaration Declaration, rows map[string]replay.Row, store replay.Store, check *Check) {
	sum, ok := sumColumn(declaration, rows, check)
	if !ok {
		return
	}
	refRows, ok := store.Tables[declaration.RefTable]
	if !ok {
		check.Counterexamples = append(check.Counterexamples, Counterexample{Table: declaration.RefTable, Message: "materialized report table not found"})
		return
	}
	var reportValue string
	for _, rowID := range sortedRowIDs(refRows) {
		reportValue = refRows[rowID][declaration.RefColumn]
		if reportValue != "" {
			break
		}
	}
	want, err := strconv.Atoi(reportValue)
	if err != nil {
		check.Counterexamples = append(check.Counterexamples, Counterexample{Table: declaration.RefTable, Column: declaration.RefColumn, Value: reportValue, Message: "materialized report value is not an integer"})
		return
	}
	if sum != want {
		check.Counterexamples = append(check.Counterexamples, Counterexample{Table: declaration.RefTable, Column: declaration.RefColumn, Value: reportValue, Message: fmt.Sprintf("materialized report differs from source sum %d", sum)})
	}
	check.Reason = "materialized report matches source column sum"
}

func checkLedgerBalance(declaration Declaration, rows map[string]replay.Row, check *Check) {
	left, ok := sumColumn(declaration, rows, check)
	if !ok {
		return
	}
	rightDecl := declaration
	rightDecl.Column = declaration.RefColumn
	right, ok := sumColumn(rightDecl, rows, check)
	if !ok {
		return
	}
	if left != right {
		check.Counterexamples = append(check.Counterexamples, Counterexample{Table: declaration.Table, Value: fmt.Sprintf("%d!=%d", left, right), Message: "ledger debit and credit sums differ"})
	}
	check.Reason = "ledger debit and credit sums balance"
}

func checkCustomerTotal(declaration Declaration, rows map[string]replay.Row, check *Check) {
	groups := map[string]int{}
	for _, rowID := range sortedRowIDs(rows) {
		group := rows[rowID][declaration.GroupColumn]
		value := rows[rowID][declaration.Column]
		if group == "" || value == "" {
			continue
		}
		parsed, err := strconv.Atoi(value)
		if err != nil {
			check.Counterexamples = append(check.Counterexamples, Counterexample{Table: declaration.Table, RowID: rowID, Column: declaration.Column, Value: value, Message: "customer-visible total is not an integer"})
			continue
		}
		check.Support++
		groups[group] += parsed
	}
	for group, total := range groups {
		if total < 0 {
			check.Counterexamples = append(check.Counterexamples, Counterexample{Table: declaration.Table, Column: declaration.GroupColumn, Value: group, Message: "customer-visible total is negative"})
		}
	}
	check.Reason = "customer-visible grouped totals are nonnegative"
}

func sumColumn(declaration Declaration, rows map[string]replay.Row, check *Check) (int, bool) {
	var sum int
	for _, rowID := range sortedRowIDs(rows) {
		value := rows[rowID][declaration.Column]
		if value == "" {
			continue
		}
		parsed, err := strconv.Atoi(value)
		if err != nil {
			check.Counterexamples = append(check.Counterexamples, Counterexample{
				Table: declaration.Table, RowID: rowID, Column: declaration.Column, Value: value,
				Message: "sum column value is not an integer",
			})
			return 0, false
		}
		check.Support++
		sum += parsed
	}
	return sum, true
}

func reportHash(report Report) string {
	report.Hash = ""
	return canonical.Hash(report)
}

func tableNames(store replay.Store) []string {
	names := make([]string, 0, len(store.Tables))
	for name := range store.Tables {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func columns(rows map[string]replay.Row) []string {
	seen := map[string]struct{}{}
	for _, row := range rows {
		for column := range row {
			seen[column] = struct{}{}
		}
	}
	out := make([]string, 0, len(seen))
	for column := range seen {
		out = append(out, column)
	}
	sort.Strings(out)
	return out
}

func columnValues(rows map[string]replay.Row, column string) []string {
	values := make([]string, 0, len(rows))
	for _, rowID := range sortedRowIDs(rows) {
		if value := rows[rowID][column]; value != "" {
			values = append(values, value)
		}
	}
	return values
}

func sortedRowIDs(rows map[string]replay.Row) []string {
	ids := make([]string, 0, len(rows))
	for id := range rows {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func allUnique(values []string) bool {
	seen := map[string]struct{}{}
	for _, value := range values {
		if _, ok := seen[value]; ok {
			return false
		}
		seen[value] = struct{}{}
	}
	return len(values) > 0
}

func allNonNegativeInts(values []string) bool {
	if len(values) == 0 {
		return false
	}
	for _, value := range values {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 0 {
			return false
		}
	}
	return true
}

func distinct(values []string) []string {
	seen := map[string]struct{}{}
	for _, value := range values {
		seen[value] = struct{}{}
	}
	out := make([]string, 0, len(seen))
	for value := range seen {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}
