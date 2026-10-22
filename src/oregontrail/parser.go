package oregontrail

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

// ParseProgram parses a complete BASIC source file into an executable Program.
func ParseProgram(source string) (*Program, error) {
	prog := &Program{
		LineStmts: make(map[int][]Statement),
		LineIndex: make(map[int]int),
	}

	rawLines := strings.Split(source, "\n")
	for lineIdx, rawLine := range rawLines {
		trimmed := strings.TrimSpace(rawLine)
		if trimmed == "" {
			continue
		}

		// Check if line starts with a line number
		i := 0
		for i < len(trimmed) && unicode.IsDigit(rune(trimmed[i])) {
			i++
		}

		if i == 0 {
			// Unnumbered line (e.g. DEFDBL, DIM, or comments at the top)
			stmts, err := parseLineStatements(trimmed)
			if err != nil {
				return nil, fmt.Errorf("source line %d: %w", lineIdx+1, err)
			}
			prog.InitStmts = append(prog.InitStmts, stmts...)
			continue
		}

		lineNum, err := strconv.Atoi(trimmed[:i])
		if err != nil {
			return nil, fmt.Errorf("invalid line number on line %d: %s", lineIdx+1, trimmed[:i])
		}

		rest := strings.TrimSpace(trimmed[i:])
		stmts, err := parseLineStatements(rest)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", lineNum, err)
		}

		prog.Lines = append(prog.Lines, lineNum)
		prog.LineStmts[lineNum] = stmts
	}

	// Sort line numbers in ascending order
	sort.Ints(prog.Lines)
	for idx, num := range prog.Lines {
		prog.LineIndex[num] = idx
	}

	// Extract all DATA statement values in line order
	for _, num := range prog.Lines {
		for _, stmt := range prog.LineStmts[num] {
			if ds, ok := stmt.(*DataStmt); ok {
				prog.DataValues = append(prog.DataValues, ds.Values...)
			}
		}
	}

	return prog, nil
}

func parseLineStatements(s string) ([]Statement, error) {
	// Split by colon outside of quotes, but REM absorbs the rest of the line.
	rawParts := splitStatements(s)
	var stmts []Statement
	for _, part := range rawParts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		stmt, err := parseSingleStatement(part)
		if err != nil {
			return nil, err
		}
		stmts = append(stmts, stmt)
	}
	return stmts, nil
}

func splitStatements(s string) []string {
	var parts []string
	var curr strings.Builder
	inQuote := false

	for i := 0; i < len(s); i++ {
		ch := s[i]
		if ch == '"' {
			inQuote = !inQuote
			curr.WriteByte(ch)
			continue
		}

		if !inQuote {
			// Check if we hit REM
			remaining := s[i:]
			if isPrefixKeyword(remaining, "REM") {
				// REM consumes everything to end of string
				curr.WriteString(remaining)
				break
			}

			if ch == ':' {
				parts = append(parts, curr.String())
				curr.Reset()
				continue
			}
		}

		curr.WriteByte(ch)
	}

	if curr.Len() > 0 {
		parts = append(parts, curr.String())
	}
	return parts
}

func isPrefixKeyword(s, kw string) bool {
	upper := strings.ToUpper(s)
	if !strings.HasPrefix(upper, kw) {
		return false
	}
	if len(upper) == len(kw) {
		return true
	}
	next := upper[len(kw)]
	return !unicode.IsLetter(rune(next)) && !unicode.IsDigit(rune(next)) && next != '$'
}

func parseSingleStatement(s string) (Statement, error) {
	trimmed := strings.TrimSpace(s)
	tokens := strings.Fields(trimmed)
	if len(tokens) == 0 {
		return &RemStmt{}, nil
	}

	head := strings.ToUpper(tokens[0])

	switch head {
	case "REM":
		return &RemStmt{Comment: trimmed[3:]}, nil

	case "DEFDBL":
		return &DefDblStmt{}, nil

	case "DIM":
		return parseDimStmt(trimmed[3:])

	case "RANDOMIZE":
		return &RandomizeStmt{}, nil

	case "PRINT":
		return parsePrintStmt(trimmed[5:])

	case "INPUT":
		varName := strings.TrimSpace(trimmed[5:])
		return &InputStmt{VarName: varName}, nil

	case "GOTO":
		target, err := strconv.Atoi(strings.TrimSpace(trimmed[4:]))
		if err != nil {
			return nil, fmt.Errorf("invalid GOTO target: %s", trimmed[4:])
		}
		return &GotoStmt{TargetLine: target}, nil

	case "GOSUB":
		target, err := strconv.Atoi(strings.TrimSpace(trimmed[5:]))
		if err != nil {
			return nil, fmt.Errorf("invalid GOSUB target: %s", trimmed[5:])
		}
		return &GosubStmt{TargetLine: target}, nil

	case "RETURN":
		return &ReturnStmt{}, nil

	case "RESTORE":
		return &RestoreStmt{}, nil

	case "SYSTEM":
		return &SystemStmt{}, nil

	case "DATA":
		return parseDataStmt(trimmed[4:])

	case "READ":
		varName := strings.TrimSpace(trimmed[4:])
		return &ReadStmt{VarName: varName}, nil

	case "ON":
		return parseOnGotoStmt(trimmed[2:])

	case "IF":
		return parseIfStmt(trimmed[2:])

	case "LET":
		return parseAssignStmt(trimmed[3:])

	default:
		// Could be implicit assignment (e.g. X1=-1 or S$(1)="BANG")
		if strings.Contains(trimmed, "=") {
			return parseAssignStmt(trimmed)
		}
		return nil, fmt.Errorf("unrecognized statement: %s", s)
	}
}

func parseDimStmt(s string) (Statement, error) {
	// e.g. S$(1 TO 5) or S$(5)
	s = strings.TrimSpace(s)
	parenOpen := strings.Index(s, "(")
	parenClose := strings.LastIndex(s, ")")
	if parenOpen == -1 || parenClose == -1 {
		return &DimStmt{Name: s, MinIndex: 1, MaxIndex: 10}, nil
	}
	name := strings.TrimSpace(s[:parenOpen])
	bounds := strings.TrimSpace(s[parenOpen+1 : parenClose])
	toIdx := strings.Index(strings.ToUpper(bounds), "TO")
	minIdx := 1
	maxIdx := 10
	if toIdx != -1 {
		minStr := strings.TrimSpace(bounds[:toIdx])
		maxStr := strings.TrimSpace(bounds[toIdx+2:])
		if m, err := strconv.Atoi(minStr); err == nil {
			minIdx = m
		}
		if m, err := strconv.Atoi(maxStr); err == nil {
			maxIdx = m
		}
	} else if m, err := strconv.Atoi(bounds); err == nil {
		maxIdx = m
	}
	return &DimStmt{Name: name, MinIndex: minIdx, MaxIndex: maxIdx}, nil
}

func parseDataStmt(s string) (Statement, error) {
	parts := strings.Split(s, ",")
	var vals []float64
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		num, err := strconv.ParseFloat(p, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid DATA number %s: %w", p, err)
		}
		vals = append(vals, num)
	}
	return &DataStmt{Values: vals}, nil
}

func parseOnGotoStmt(s string) (Statement, error) {
	// e.g. D3 GOTO 1310,1330,... or D3-10 GOTO ...
	gotoIdx := strings.Index(strings.ToUpper(s), "GOTO")
	if gotoIdx == -1 {
		return nil, fmt.Errorf("missing GOTO in ON statement: %s", s)
	}
	exprStr := strings.TrimSpace(s[:gotoIdx])
	targetsStr := strings.TrimSpace(s[gotoIdx+4:])

	expr, err := parseExpr(exprStr)
	if err != nil {
		return nil, fmt.Errorf("invalid ON expr %s: %w", exprStr, err)
	}

	var targets []int
	for _, part := range strings.Split(targetsStr, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		t, err := strconv.Atoi(part)
		if err != nil {
			return nil, fmt.Errorf("invalid target %s in ON GOTO: %w", part, err)
		}
		targets = append(targets, t)
	}

	return &OnGotoStmt{Expr: expr, Targets: targets}, nil
}

func parseIfStmt(s string) (Statement, error) {
	// e.g. C$="NO" THEN 690 or B1<B3 THEN B1=B1+86400
	thenIdx := strings.Index(strings.ToUpper(s), "THEN")
	if thenIdx == -1 {
		return nil, fmt.Errorf("missing THEN in IF statement: %s", s)
	}
	condStr := strings.TrimSpace(s[:thenIdx])
	targetStr := strings.TrimSpace(s[thenIdx+4:])

	cond, err := parseExpr(condStr)
	if err != nil {
		return nil, fmt.Errorf("invalid IF condition %s: %w", condStr, err)
	}

	// Check if target is a line number
	targetTokens := strings.Fields(targetStr)
	if len(targetTokens) > 0 && unicode.IsDigit(rune(targetTokens[0][0])) {
		lineNum, err := strconv.Atoi(targetTokens[0])
		if err == nil {
			return &IfStmt{Cond: cond, TargetLine: lineNum}, nil
		}
	}

	// Target is a statement (like B1=B1+86400)
	targetStmt, err := parseSingleStatement(targetStr)
	if err != nil {
		return nil, fmt.Errorf("invalid THEN statement %s: %w", targetStr, err)
	}
	return &IfStmt{Cond: cond, TargetStmt: targetStmt}, nil
}

func parseAssignStmt(s string) (Statement, error) {
	// e.g. A=200 or S$(1)="BANG"
	eqIdx := strings.Index(s, "=")
	if eqIdx == -1 {
		return nil, fmt.Errorf("missing = in assignment: %s", s)
	}
	target := strings.TrimSpace(s[:eqIdx])
	exprStr := strings.TrimSpace(s[eqIdx+1:])

	expr, err := parseExpr(exprStr)
	if err != nil {
		return nil, fmt.Errorf("invalid assignment expr in %s: %w", s, err)
	}

	// Check if target has array index
	parenOpen := strings.Index(target, "(")
	parenClose := strings.LastIndex(target, ")")
	if parenOpen != -1 && parenClose != -1 && parenClose > parenOpen {
		arrName := strings.TrimSpace(target[:parenOpen])
		idxExprStr := strings.TrimSpace(target[parenOpen+1 : parenClose])
		idxExpr, err := parseExpr(idxExprStr)
		if err != nil {
			return nil, fmt.Errorf("invalid array index in %s: %w", target, err)
		}
		return &AssignStmt{VarName: arrName, Index: idxExpr, Expr: expr}, nil
	}

	return &AssignStmt{VarName: target, Expr: expr}, nil
}

func parsePrintStmt(s string) (Statement, error) {
	var items []PrintItem
	i := 0
	n := len(s)

	for i < n {
		ch := s[i]
		if unicode.IsSpace(rune(ch)) {
			i++
			continue
		}

		if ch == ';' {
			items = append(items, PrintItem{IsSemicolon: true})
			i++
			continue
		}

		if ch == ',' {
			items = append(items, PrintItem{IsComma: true})
			i++
			continue
		}

		// Check for TAB(expr)
		remaining := s[i:]
		if isPrefixKeyword(remaining, "TAB") {
			parenOpen := strings.Index(remaining, "(")
			if parenOpen != -1 {
				parenClose := findMatchingParen(remaining[parenOpen:])
				if parenClose != -1 {
					tabExprStr := remaining[parenOpen+1 : parenOpen+parenClose]
					tabExpr, err := parseExpr(tabExprStr)
					if err != nil {
						return nil, fmt.Errorf("invalid TAB expr %s: %w", tabExprStr, err)
					}
					items = append(items, PrintItem{IsTab: true, TabExpr: tabExpr})
					i += parenOpen + parenClose + 1
					continue
				}
			}
		}

		// Parse expression until next delimiter (, or ;) outside quotes and parens
		start := i
		inQuote := false
		parenDepth := 0
		for i < n {
			c := s[i]
			if c == '"' {
				inQuote = !inQuote
			} else if !inQuote {
				if c == '(' {
					parenDepth++
				} else if c == ')' {
					parenDepth--
				} else if (c == ';' || c == ',') && parenDepth == 0 {
					break
				}
			}
			i++
		}

		exprStr := strings.TrimSpace(s[start:i])
		if exprStr != "" {
			expr, err := parseExpr(exprStr)
			if err != nil {
				return nil, fmt.Errorf("invalid PRINT expr %s: %w", exprStr, err)
			}
			items = append(items, PrintItem{Expr: expr})
		}
	}

	return &PrintStmt{Items: items}, nil
}

func findMatchingParen(s string) int {
	depth := 0
	inQuote := false
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if ch == '"' {
			inQuote = !inQuote
		} else if !inQuote {
			switch ch {
			case '(':
				depth++
			case ')':
				depth--
				if depth == 0 {
					return i
				}
			}
		}
	}
	return -1
}

// Pratt parser for expressions.
type tokenType int

const (
	tokEOF tokenType = iota
	tokNum
	tokStr
	tokIdent
	tokOp
	tokLParen
	tokRParen
	tokComma
)

type token struct {
	typ tokenType
	val string
	num float64
}

func tokenizeExpr(s string) ([]token, error) {
	var tokens []token
	i := 0
	n := len(s)

	for i < n {
		ch := s[i]
		if unicode.IsSpace(rune(ch)) {
			i++
			continue
		}

		if ch == '"' {
			start := i + 1
			i++
			for i < n && s[i] != '"' {
				i++
			}
			if i >= n {
				return nil, errors.New("unterminated string literal")
			}
			tokens = append(tokens, token{typ: tokStr, val: s[start:i]})
			i++
			continue
		}

		if ch == '(' {
			tokens = append(tokens, token{typ: tokLParen, val: "("})
			i++
			continue
		}
		if ch == ')' {
			tokens = append(tokens, token{typ: tokRParen, val: ")"})
			i++
			continue
		}
		if ch == ',' {
			tokens = append(tokens, token{typ: tokComma, val: ","})
			i++
			continue
		}

		// Two-character operators: <=, >=, <>, !=
		if i+1 < n {
			two := s[i : i+2]
			if two == "<=" || two == ">=" || two == "<>" || two == "!=" {
				tokens = append(tokens, token{typ: tokOp, val: two})
				i += 2
				continue
			}
		}

		// Single character operators: +, -, *, /, ^, =, <, >
		if strings.ContainsRune("+-*/^=<>", rune(ch)) {
			tokens = append(tokens, token{typ: tokOp, val: string(ch)})
			i++
			continue
		}

		// Number (including starting with .)
		if unicode.IsDigit(rune(ch)) || (ch == '.' && i+1 < n && unicode.IsDigit(rune(s[i+1]))) {
			start := i
			for i < n && (unicode.IsDigit(rune(s[i])) || s[i] == '.') {
				i++
			}
			numStr := s[start:i]
			num, err := strconv.ParseFloat(numStr, 64)
			if err != nil {
				return nil, fmt.Errorf("invalid number %s: %w", numStr, err)
			}
			tokens = append(tokens, token{typ: tokNum, val: numStr, num: num})
			continue
		}

		// Identifier (letter followed by letters/digits/$, e.g. INT, CHR$, S$, C$, D9, M1)
		if unicode.IsLetter(rune(ch)) {
			start := i
			for i < n && (unicode.IsLetter(rune(s[i])) || unicode.IsDigit(rune(s[i])) || s[i] == '$') {
				i++
			}
			tokens = append(tokens, token{typ: tokIdent, val: s[start:i]})
			continue
		}

		return nil, fmt.Errorf("unexpected character in expression: %c", ch)
	}

	tokens = append(tokens, token{typ: tokEOF})
	return tokens, nil
}

func parseExpr(s string) (Expr, error) {
	tokens, err := tokenizeExpr(s)
	if err != nil {
		return nil, err
	}
	p := &exprParser{tokens: tokens, pos: 0}
	expr, err := p.parseBinary(0)
	if err != nil {
		return nil, err
	}
	if p.current().typ != tokEOF {
		return nil, fmt.Errorf("unexpected trailing token in expression: %s", p.current().val)
	}
	return expr, nil
}

type exprParser struct {
	tokens []token
	pos    int
}

func (p *exprParser) current() token {
	if p.pos < len(p.tokens) {
		return p.tokens[p.pos]
	}
	return token{typ: tokEOF}
}

func (p *exprParser) advance() token {
	tok := p.current()
	if p.pos < len(p.tokens) {
		p.pos++
	}
	return tok
}

func getPrecedence(op string) int {
	switch op {
	case "=", "<>", "!=", "<", ">", "<=", ">=":
		return 1
	case "+", "-":
		return 2
	case "*", "/":
		return 3
	case "^":
		return 4
	default:
		return 0
	}
}

func (p *exprParser) parseBinary(minPrec int) (Expr, error) {
	left, err := p.parseUnary()
	if err != nil {
		return nil, err
	}

	for {
		tok := p.current()
		if tok.typ != tokOp {
			break
		}
		prec := getPrecedence(tok.val)
		if prec < minPrec {
			break
		}
		p.advance()
		nextMinPrec := prec + 1
		if tok.val == "^" {
			nextMinPrec = prec // right associative
		}
		right, err := p.parseBinary(nextMinPrec)
		if err != nil {
			return nil, err
		}
		left = &BinaryExpr{Op: tok.val, Left: left, Right: right}
	}

	return left, nil
}

func (p *exprParser) parseUnary() (Expr, error) {
	tok := p.current()
	if tok.typ == tokOp && (tok.val == "+" || tok.val == "-") {
		p.advance()
		sub, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return &UnaryExpr{Op: rune(tok.val[0]), Expr: sub}, nil
	}
	return p.parsePrimary()
}

func (p *exprParser) parsePrimary() (Expr, error) {
	tok := p.current()
	switch tok.typ {
	case tokNum:
		p.advance()
		return &NumLit{Val: tok.num}, nil

	case tokStr:
		p.advance()
		return &StrLit{Val: tok.val}, nil

	case tokLParen:
		p.advance()
		expr, err := p.parseBinary(0)
		if err != nil {
			return nil, err
		}
		if p.current().typ != tokRParen {
			return nil, fmt.Errorf("expected ')' but got %s", p.current().val)
		}
		p.advance()
		return expr, nil

	case tokIdent:
		ident := p.advance().val
		upper := strings.ToUpper(ident)

		// Check if followed by '(' (function or array)
		if p.current().typ == tokLParen {
			p.advance()
			arg, err := p.parseBinary(0)
			if err != nil {
				return nil, err
			}
			if p.current().typ != tokRParen {
				return nil, fmt.Errorf("expected ')' closing %s( but got %s", ident, p.current().val)
			}
			p.advance()

			switch upper {
			case "INT", "CHR$", "TAB":
				return &FuncCallExpr{Name: upper, Arg: arg}, nil
			default:
				// Array access (e.g. S$(S6))
				return &ArrayAccessExpr{Name: ident, Index: arg}, nil
			}
		}

		return &VarExpr{Name: ident}, nil

	default:
		return nil, fmt.Errorf("unexpected token in primary expr: %s (type %d)", tok.val, tok.typ)
	}
}
