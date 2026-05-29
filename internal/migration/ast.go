package migration

type StatementAST struct {
	Kind     string
	Table    string
	HasWhere bool
	Clauses  []Clause
}

type Clause struct {
	Kind  string
	Start int
	End   int
}

func parseStatementAST(tokens []string) StatementAST {
	ast := StatementAST{
		Kind:     firstToken(tokens),
		HasWhere: containsToken(tokens, "where"),
		Clauses:  parseClauses(tokens),
	}
	switch ast.Kind {
	case "update":
		ast.Table = tokenAfter(tokens, "update")
	case "delete":
		ast.Table = tableAfterDelete(tokens)
	case "alter":
		ast.Table = tableAfterAlter(tokens)
	case "drop", "truncate":
		ast.Table = tableAfterDropOrTruncate(tokens, ast.Kind)
	case "insert":
		ast.Table = tableAfterInsert(tokens)
	case "replace":
		ast.Table = tableAfterReplace(tokens)
	case "create":
		ast.Table = tableAfterCreate(tokens)
	case "select":
		ast.Table = tokenAfter(tokens, "from")
	}
	return ast
}

func parseClauses(tokens []string) []Clause {
	var starts []int
	for i, token := range tokens {
		if isClauseToken(token) {
			starts = append(starts, i)
		}
	}
	clauses := make([]Clause, 0, len(starts))
	for i, start := range starts {
		end := len(tokens)
		if i+1 < len(starts) {
			end = starts[i+1]
		}
		clauses = append(clauses, Clause{Kind: tokens[start], Start: start, End: end})
	}
	return clauses
}

func isClauseToken(token string) bool {
	switch token {
	case "set", "from", "where", "values", "returning", "using", "join", "on", "order", "limit", "top":
		return true
	default:
		return false
	}
}
