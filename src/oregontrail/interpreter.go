package oregontrail

import (
	"errors"
	"fmt"
	"math"
	"math/rand"
	"strconv"
	"strings"
	"time"
)

// ValueType distinguishes numeric and string values.
type ValueType int

const (
	ValNum ValueType = iota
	ValStr
)

// Value is a runtime value in the BASIC interpreter.
type Value struct {
	Type ValueType
	Num  float64
	Str  string
}

func numVal(n float64) Value {
	return Value{Type: ValNum, Num: n}
}

func strVal(s string) Value {
	return Value{Type: ValStr, Str: s}
}

func (v Value) isTrue() bool {
	if v.Type == ValNum {
		return v.Num != 0
	}
	return v.Str != ""
}

func (v Value) String() string {
	if v.Type == ValStr {
		return v.Str
	}
	// In BASIC, integers are printed without decimals.
	if v.Num == math.Trunc(v.Num) && !math.IsInf(v.Num, 0) && !math.IsNaN(v.Num) {
		return strconv.FormatInt(int64(v.Num), 10)
	}
	return strconv.FormatFloat(v.Num, 'f', -1, 64)
}

// Expr represents an AST expression.
type Expr interface {
	Eval(env *Env) (Value, error)
}

// NumLit represents a numeric literal.
type NumLit struct {
	Val float64
}

func (e *NumLit) Eval(env *Env) (Value, error) {
	return numVal(e.Val), nil
}

// StrLit represents a string literal.
type StrLit struct {
	Val string
}

func (e *StrLit) Eval(env *Env) (Value, error) {
	return strVal(e.Val), nil
}

// VarExpr represents a variable lookup (scalar numeric or string).
type VarExpr struct {
	Name string
}

func (e *VarExpr) Eval(env *Env) (Value, error) {
	name := strings.ToUpper(e.Name)
	if strings.HasSuffix(name, "$") {
		return strVal(env.strs[name]), nil
	}
	switch name {
	case "RND":
		return numVal(env.rng.Float64()), nil
	case "TIMER":
		if env.TimerFunc != nil {
			return numVal(env.TimerFunc()), nil
		}
		now := time.Now()
		secs := float64(now.Hour()*3600+now.Minute()*60+now.Second()) + float64(now.Nanosecond())/1e9
		return numVal(secs), nil
	default:
		return numVal(env.nums[name]), nil
	}
}

// ArrayAccessExpr represents indexing an array like S$(S6).
type ArrayAccessExpr struct {
	Name  string
	Index Expr
}

func (e *ArrayAccessExpr) Eval(env *Env) (Value, error) {
	idxVal, err := e.Index.Eval(env)
	if err != nil {
		return Value{}, err
	}
	idx := int(math.Round(idxVal.Num))
	name := strings.ToUpper(e.Name)
	arr := env.strArrays[name]
	if arr == nil {
		return strVal(""), nil
	}
	return strVal(arr[idx]), nil
}

// UnaryExpr represents a unary operation like -A.
type UnaryExpr struct {
	Op   rune
	Expr Expr
}

func (e *UnaryExpr) Eval(env *Env) (Value, error) {
	v, err := e.Expr.Eval(env)
	if err != nil {
		return Value{}, err
	}
	if e.Op == '-' {
		return numVal(-v.Num), nil
	}
	return v, nil
}

// BinaryExpr represents binary arithmetic and relational operations.
type BinaryExpr struct {
	Op    string
	Left  Expr
	Right Expr
}

func (e *BinaryExpr) Eval(env *Env) (Value, error) {
	l, err := e.Left.Eval(env)
	if err != nil {
		return Value{}, err
	}
	r, err := e.Right.Eval(env)
	if err != nil {
		return Value{}, err
	}

	// Relational operations on strings:
	if l.Type == ValStr || r.Type == ValStr {
		lStr := l.Str
		rStr := r.Str
		switch e.Op {
		case "=":
			if lStr == rStr {
				return numVal(1), nil
			}
			return numVal(0), nil
		case "<>", "!=":
			if lStr != rStr {
				return numVal(1), nil
			}
			return numVal(0), nil
		default:
			return numVal(0), fmt.Errorf("unsupported string op: %s", e.Op)
		}
	}

	// Numeric operations:
	switch e.Op {
	case "+":
		return numVal(l.Num + r.Num), nil
	case "-":
		return numVal(l.Num - r.Num), nil
	case "*":
		return numVal(l.Num * r.Num), nil
	case "/":
		if r.Num == 0 {
			return numVal(0), nil
		}
		return numVal(l.Num / r.Num), nil
	case "^":
		return numVal(math.Pow(l.Num, r.Num)), nil
	case "=":
		if l.Num == r.Num {
			return numVal(1), nil
		}
		return numVal(0), nil
	case "<>", "!=":
		if l.Num != r.Num {
			return numVal(1), nil
		}
		return numVal(0), nil
	case "<":
		if l.Num < r.Num {
			return numVal(1), nil
		}
		return numVal(0), nil
	case ">":
		if l.Num > r.Num {
			return numVal(1), nil
		}
		return numVal(0), nil
	case "<=":
		if l.Num <= r.Num {
			return numVal(1), nil
		}
		return numVal(0), nil
	case ">=":
		if l.Num >= r.Num {
			return numVal(1), nil
		}
		return numVal(0), nil
	default:
		return Value{}, fmt.Errorf("unknown binary operator: %s", e.Op)
	}
}

// FuncCallExpr represents built-in functions: INT, CHR$, TAB.
type FuncCallExpr struct {
	Name string
	Arg  Expr
}

func (e *FuncCallExpr) Eval(env *Env) (Value, error) {
	name := strings.ToUpper(e.Name)
	arg, err := e.Arg.Eval(env)
	if err != nil {
		return Value{}, err
	}
	switch name {
	case "INT":
		return numVal(math.Floor(arg.Num)), nil
	case "CHR$":
		ch := rune(int(arg.Num))
		return strVal(string(ch)), nil
	case "TAB":
		// TAB in PRINT will be handled by PrintItem, but if evaluated here:
		return numVal(arg.Num), nil
	default:
		return Value{}, fmt.Errorf("unknown function: %s", name)
	}
}

// Statement represents a BASIC executable statement.
type Statement interface {
	Exec(env *Env) (jumpTarget int, err error)
}

// RemStmt is a comment.
type RemStmt struct {
	Comment string
}

func (s *RemStmt) Exec(env *Env) (int, error) {
	return 0, nil
}

// DefDblStmt sets precision (no-op).
type DefDblStmt struct{}

func (s *DefDblStmt) Exec(env *Env) (int, error) {
	return 0, nil
}

// DimStmt dimensions array.
type DimStmt struct {
	Name     string
	MinIndex int
	MaxIndex int
}

func (s *DimStmt) Exec(env *Env) (int, error) {
	name := strings.ToUpper(s.Name)
	if env.strArrays[name] == nil {
		env.strArrays[name] = make(map[int]string)
	}
	return 0, nil
}

// RandomizeStmt seeds random generator.
type RandomizeStmt struct{}

func (s *RandomizeStmt) Exec(env *Env) (int, error) {
	if env.rngSeeded {
		return 0, nil
	}
	env.rng = rand.New(rand.NewSource(time.Now().UnixNano()))
	env.rngSeeded = true
	return 0, nil
}

// PrintItem is one component in a PRINT statement.
type PrintItem struct {
	IsComma     bool
	IsSemicolon bool
	IsTab       bool
	TabExpr     Expr
	Expr        Expr
}

// PrintStmt prints values.
type PrintStmt struct {
	Items []PrintItem
}

func (s *PrintStmt) Exec(env *Env) (int, error) {
	if len(s.Items) == 0 {
		env.output.WriteString("\n")
		env.curCol = 0
		return 0, nil
	}

	trailingSep := false
	for _, item := range s.Items {
		if item.IsComma {
			// Tab stop every 14 columns
			nextCol := ((env.curCol / 14) + 1) * 14
			pad := nextCol - env.curCol
			if pad < 1 {
				pad = 14
			}
			env.output.WriteString(strings.Repeat(" ", pad))
			env.curCol += pad
			trailingSep = true
		} else if item.IsSemicolon {
			trailingSep = true
		} else if item.IsTab {
			tabVal, err := item.TabExpr.Eval(env)
			if err != nil {
				return 0, err
			}
			targetCol := int(tabVal.Num) - 1 // 1-indexed TAB in BASIC
			if targetCol < 0 {
				targetCol = 0
			}
			if targetCol > env.curCol {
				pad := targetCol - env.curCol
				env.output.WriteString(strings.Repeat(" ", pad))
				env.curCol = targetCol
			}
			trailingSep = false
		} else if item.Expr != nil {
			val, err := item.Expr.Eval(env)
			if err != nil {
				return 0, err
			}
			var str string
			if val.Type == ValStr {
				str = val.Str
			} else {
				// In BASIC, numbers output with a leading space if positive,
				// and a trailing space.
				numStr := val.String()
				if val.Num >= 0 {
					str = " " + numStr + " "
				} else {
					str = numStr + " "
				}
			}
			env.output.WriteString(str)
			env.curCol += len(str)
			trailingSep = false
		}
	}

	if !trailingSep {
		env.output.WriteString("\n")
		env.curCol = 0
	}
	return 0, nil
}

// InputStmt pauses execution for user input.
type InputStmt struct {
	VarName string
}

func (s *InputStmt) Exec(env *Env) (int, error) {
	env.waitingVar = strings.ToUpper(s.VarName)
	env.isWaitingInput = true
	return 0, nil
}

// AssignStmt assigns an expression to a scalar or array variable.
type AssignStmt struct {
	VarName string
	Index   Expr // nil for scalar
	Expr    Expr
}

func (s *AssignStmt) Exec(env *Env) (int, error) {
	val, err := s.Expr.Eval(env)
	if err != nil {
		return 0, err
	}
	name := strings.ToUpper(s.VarName)
	if s.Index != nil {
		idxVal, err := s.Index.Eval(env)
		if err != nil {
			return 0, err
		}
		idx := int(math.Round(idxVal.Num))
		arr := env.strArrays[name]
		if arr == nil {
			arr = make(map[int]string)
			env.strArrays[name] = arr
		}
		arr[idx] = val.Str
		return 0, nil
	}

	if strings.HasSuffix(name, "$") {
		env.strs[name] = val.Str
	} else {
		env.nums[name] = val.Num
	}
	return 0, nil
}

// GotoStmt jumps to a line number.
type GotoStmt struct {
	TargetLine int
}

func (s *GotoStmt) Exec(env *Env) (int, error) {
	return s.TargetLine, nil
}

// GosubStmt pushes return address and jumps.
type GosubStmt struct {
	TargetLine int
}

func (s *GosubStmt) Exec(env *Env) (int, error) {
	env.callStack = append(env.callStack, env.nextAddr)
	return s.TargetLine, nil
}

// ReturnStmt returns from a subroutine.
type ReturnStmt struct{}

func (s *ReturnStmt) Exec(env *Env) (int, error) {
	if len(env.callStack) == 0 {
		return 0, errors.New("RETURN without GOSUB")
	}
	ret := env.callStack[len(env.callStack)-1]
	env.callStack = env.callStack[:len(env.callStack)-1]
	env.jumpToAddr = &ret
	return -1, nil // -1 signals jump to full address
}

// IfStmt executes target if condition is true.
type IfStmt struct {
	Cond       Expr
	TargetLine int
	TargetStmt Statement // non-nil if statement follows THEN instead of line number
}

func (s *IfStmt) Exec(env *Env) (int, error) {
	condVal, err := s.Cond.Eval(env)
	if err != nil {
		return 0, err
	}
	if condVal.isTrue() {
		if s.TargetStmt != nil {
			return s.TargetStmt.Exec(env)
		}
		return s.TargetLine, nil
	}
	return 0, nil
}

// OnGotoStmt branches to targets[n-1].
type OnGotoStmt struct {
	Expr    Expr
	Targets []int
}

func (s *OnGotoStmt) Exec(env *Env) (int, error) {
	v, err := s.Expr.Eval(env)
	if err != nil {
		return 0, err
	}
	idx := int(math.Floor(v.Num))
	if idx >= 1 && idx <= len(s.Targets) {
		return s.Targets[idx-1], nil
	}
	return 0, nil
}

// DataStmt registers data items.
type DataStmt struct {
	Values []float64
}

func (s *DataStmt) Exec(env *Env) (int, error) {
	return 0, nil
}

// ReadStmt reads from DATA.
type ReadStmt struct {
	VarName string
}

func (s *ReadStmt) Exec(env *Env) (int, error) {
	if env.dataIndex >= len(env.data) {
		return 0, errors.New("READ out of DATA")
	}
	val := env.data[env.dataIndex]
	env.dataIndex++
	name := strings.ToUpper(s.VarName)
	env.nums[name] = val
	return 0, nil
}

// RestoreStmt resets data index to 0.
type RestoreStmt struct{}

func (s *RestoreStmt) Exec(env *Env) (int, error) {
	env.dataIndex = 0
	return 0, nil
}

// SystemStmt halts execution.
type SystemStmt struct{}

func (s *SystemStmt) Exec(env *Env) (int, error) {
	env.terminated = true
	return 0, nil
}

// CodeAddr identifies a statement in the program.
type CodeAddr struct {
	LineIndex int
	StmtIndex int
}

// Program holds parsed line numbers and statements.
type Program struct {
	Lines       []int
	LineStmts   map[int][]Statement
	LineIndex   map[int]int
	DataValues  []float64
	InitStmts   []Statement // Unnumbered statements like DEFDBL, DIM executed before line 10
}

// Env represents runtime state.
type Env struct {
	prog           *Program
	nums           map[string]float64
	strs           map[string]string
	strArrays      map[string]map[int]string
	data           []float64
	dataIndex      int
	callStack      []CodeAddr
	nextAddr       CodeAddr
	jumpToAddr     *CodeAddr
	curLineIdx     int
	curStmtIdx     int
	isWaitingInput bool
	waitingVar     string
	terminated     bool
	output         strings.Builder
	curCol         int
	rng            *rand.Rand
	rngSeeded      bool
	TimerFunc      func() float64
}

// NewEnv creates a new execution environment for a program.
func NewEnv(prog *Program, seed int64) *Env {
	r := rand.New(rand.NewSource(seed))
	return &Env{
		prog:       prog,
		nums:       make(map[string]float64),
		strs:       make(map[string]string),
		strArrays:  make(map[string]map[int]string),
		data:       prog.DataValues,
		dataIndex:  0,
		callStack:  nil,
		curLineIdx: 0,
		curStmtIdx: 0,
		rng:        r,
	}
}

// SetTimerFunc overrides the TIMER built-in function.
func (env *Env) SetTimerFunc(f func() float64) {
	env.TimerFunc = f
}

// IsWaitingInput reports whether the interpreter is paused at an INPUT statement.
func (env *Env) IsWaitingInput() bool {
	return env.isWaitingInput
}

// WaitingVar returns the variable name being waited on.
func (env *Env) WaitingVar() string {
	return env.waitingVar
}

// IsTerminated reports whether the program has finished.
func (env *Env) IsTerminated() bool {
	return env.terminated
}

// OutputDrain returns all accumulated text output and clears the buffer.
func (env *Env) OutputDrain() string {
	out := env.output.String()
	env.output.Reset()
	return out
}

// OutputPeek returns the accumulated output without clearing it.
func (env *Env) OutputPeek() string {
	return env.output.String()
}

// ProvideInput feeds a user input to the waiting INPUT statement and resumes.
func (env *Env) ProvideInput(inputStr string) error {
	if !env.isWaitingInput {
		return errors.New("interpreter is not waiting for input")
	}
	varName := env.waitingVar
	env.isWaitingInput = false
	env.waitingVar = ""

	trimmed := strings.TrimSpace(inputStr)
	if strings.HasSuffix(varName, "$") {
		env.strs[varName] = strings.ToUpper(trimmed)
	} else {
		num, err := strconv.ParseFloat(trimmed, 64)
		if err != nil {
			// If invalid numeric input in Oregon Trail, default to 0 or re-prompt
			num = 0
		}
		env.nums[varName] = num
	}

	return env.Resume()
}

// Start executes unnumbered init statements and begins running the program.
func (env *Env) Start() error {
	for _, stmt := range env.prog.InitStmts {
		if _, err := stmt.Exec(env); err != nil {
			return err
		}
	}
	env.curLineIdx = 0
	env.curStmtIdx = 0
	return env.Resume()
}

// Resume runs statements until an INPUT statement is hit or the program terminates.
func (env *Env) Resume() error {
	for !env.terminated && !env.isWaitingInput {
		if env.curLineIdx >= len(env.prog.Lines) {
			env.terminated = true
			break
		}

		lineNum := env.prog.Lines[env.curLineIdx]
		stmts := env.prog.LineStmts[lineNum]

		if env.curStmtIdx >= len(stmts) {
			env.curLineIdx++
			env.curStmtIdx = 0
			continue
		}

		stmt := stmts[env.curStmtIdx]

		// Compute next default statement address
		if env.curStmtIdx+1 < len(stmts) {
			env.nextAddr = CodeAddr{LineIndex: env.curLineIdx, StmtIndex: env.curStmtIdx + 1}
		} else {
			env.nextAddr = CodeAddr{LineIndex: env.curLineIdx + 1, StmtIndex: 0}
		}

		jumpTarget, err := stmt.Exec(env)
		if err != nil {
			return fmt.Errorf("line %d: %w", lineNum, err)
		}

		if env.jumpToAddr != nil {
			env.curLineIdx = env.jumpToAddr.LineIndex
			env.curStmtIdx = env.jumpToAddr.StmtIndex
			env.jumpToAddr = nil
		} else if jumpTarget > 0 {
			targetLineIdx, ok := env.prog.LineIndex[jumpTarget]
			if !ok {
				return fmt.Errorf("jump to undefined line %d from line %d", jumpTarget, lineNum)
			}
			env.curLineIdx = targetLineIdx
			env.curStmtIdx = 0
		} else {
			env.curStmtIdx++
		}
	}
	return nil
}

// NumVar returns the current float value of a variable.
func (env *Env) NumVar(name string) float64 {
	return env.nums[strings.ToUpper(name)]
}

// StrVar returns the current string value of a variable.
func (env *Env) StrVar(name string) string {
	return env.strs[strings.ToUpper(name)]
}
