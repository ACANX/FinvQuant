package backtest

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode"
)

// ---------------------------------------------------------------------
// 信号表达式引擎：自研轻量表达式解析与求值（无第三方依赖）。
//
// 支持语法：
//   - 数值：123、1.5
//   - 指标引用：ma_fast、rsi14（取当前 bar 位置的值）
//   - 字段引用：open/high/low/close/volume/turnover（当前 bar）
//   - 算术：+ - * / （如 close - ma_slow）
//   - 比较：== != > >= < <= （NaN 参与比较恒为 false）
//   - 逻辑：AND OR NOT（大小写不敏感）
//   - 内置函数：
//       cross_up(a, b)     a 上穿 b（a[i]>b[i] 且 a[i-1]<=b[i-1]）
//       cross_down(a, b)   a 下穿 b
//       ref(id, n)         n 根 bar 前的值（n>=0，编译期要求 n 为非负常量，运行期 n<0 返回 NaN）
//       highest(id, n)     最近 n 根（含当前）最大值
//       lowest(id, n)      最近 n 根（含当前）最小值
//       abs(x)             绝对值
//   - 括号
//
// 顶层表达式必须求值为布尔（供买卖信号使用）。
//
// 安全约束（无未来函数承诺）：
//   - ref/highest/lowest 的偏移参数 n 必须 >= 0（词法不支持负数字面量，但算术可构造负数，
//     如 ref(close, 0-5)）；编译期对常量参数做下界校验，运行期对动态值再做 n<0 防御（返回 NaN）；
//   - 表达式嵌套深度受限（maxExprDepth=64），防止深递归导致栈溢出。
// ---------------------------------------------------------------------

// maxExprDepth 表达式 AST 最大嵌套深度（防递归栈溢出）。
const maxExprDepth = 64

// exprNode AST 节点。
type exprNode interface{}

type numNode struct{ value float64 }
type identNode struct{ name string }
type binaryNode struct {
	op    string
	left  exprNode
	right exprNode
}
type unaryNode struct {
	op   string
	node exprNode
}
type callNode struct {
	name string
	args []exprNode
}

// EvalContext 求值上下文（回测引擎逐 bar 提供）。
type EvalContext struct {
	At         int                  // 当前 bar 索引
	Fields     map[string][]float64 // 字段序列（open/high/low/close/volume/turnover）
	Indicators map[string][]float64 // 指标序列（id → 序列）
}

func (c *EvalContext) series(name string) ([]float64, bool) {
	if s, ok := c.Indicators[name]; ok {
		return s, true
	}
	if s, ok := c.Fields[name]; ok {
		return s, true
	}
	return nil, false
}

func (c *EvalContext) at(name string) (float64, bool) {
	s, ok := c.series(name)
	if !ok {
		return 0, false
	}
	if c.At < 0 || c.At >= len(s) {
		return 0, false
	}
	return s[c.At], true
}

// ---------------------------------------------------------------------
// 词法分析
// ---------------------------------------------------------------------

type tokenKind int

const (
	tokEOF tokenKind = iota
	tokNumber
	tokIdent
	tokOp // == != > >= < <= + - * /
	tokLParen
	tokRParen
	tokComma
	tokAnd
	tokOr
	tokNot
)

type token struct {
	kind  tokenKind
	text  string
	value float64
}

type lexer struct {
	src  string
	pos  int
	toks []token
}

func lexExpr(src string) ([]token, error) {
	l := &lexer{src: src}
	for l.pos < len(l.src) {
		ch := l.src[l.pos]
		switch {
		case ch == ' ' || ch == '\t' || ch == '\r' || ch == '\n':
			l.pos++
		case ch == '(':
			l.toks = append(l.toks, token{kind: tokLParen, text: "("})
			l.pos++
		case ch == ')':
			l.toks = append(l.toks, token{kind: tokRParen, text: ")"})
			l.pos++
		case ch == ',':
			l.toks = append(l.toks, token{kind: tokComma, text: ","})
			l.pos++
		case ch == '+' || ch == '-' || ch == '*' || ch == '/':
			l.toks = append(l.toks, token{kind: tokOp, text: string(ch)})
			l.pos++
		case ch == '=' || ch == '!' || ch == '>' || ch == '<':
			start := l.pos
			l.pos++
			if l.pos < len(l.src) && l.src[l.pos] == '=' {
				l.pos++
			}
			l.toks = append(l.toks, token{kind: tokOp, text: l.src[start:l.pos]})
		case unicode.IsDigit(rune(ch)) || (ch == '.' && l.pos+1 < len(l.src) && unicode.IsDigit(rune(l.src[l.pos+1]))):
			start := l.pos
			for l.pos < len(l.src) && (unicode.IsDigit(rune(l.src[l.pos])) || l.src[l.pos] == '.') {
				l.pos++
			}
			num, err := strconv.ParseFloat(l.src[start:l.pos], 64)
			if err != nil {
				return nil, fmt.Errorf("非法数字 %q", l.src[start:l.pos])
			}
			l.toks = append(l.toks, token{kind: tokNumber, text: l.src[start:l.pos], value: num})
		case unicode.IsLetter(rune(ch)) || ch == '_':
			start := l.pos
			for l.pos < len(l.src) && (unicode.IsLetter(rune(l.src[l.pos])) || unicode.IsDigit(rune(l.src[l.pos])) || l.src[l.pos] == '_') {
				l.pos++
			}
			word := l.src[start:l.pos]
			switch strings.ToUpper(word) {
			case "AND":
				l.toks = append(l.toks, token{kind: tokAnd, text: word})
			case "OR":
				l.toks = append(l.toks, token{kind: tokOr, text: word})
			case "NOT":
				l.toks = append(l.toks, token{kind: tokNot, text: word})
			default:
				l.toks = append(l.toks, token{kind: tokIdent, text: word})
			}
		default:
			return nil, fmt.Errorf("非法字符 %q（位置 %d）", string(ch), l.pos)
		}
	}
	l.toks = append(l.toks, token{kind: tokEOF})
	return l.toks, nil
}

// ---------------------------------------------------------------------
// 语法分析（递归下降）
// ---------------------------------------------------------------------

type parser struct {
	toks []token
	pos  int
}

func parseExpr(src string) (exprNode, error) {
	toks, err := lexExpr(src)
	if err != nil {
		return nil, err
	}
	p := &parser{toks: toks}
	node, err := p.parseOrDepth(0)
	if err != nil {
		return nil, err
	}
	if p.peek().kind != tokEOF {
		return nil, fmt.Errorf("表达式尾部存在多余内容 %q", p.peek().text)
	}
	return node, nil
}

// parseOrDepth 带深度限制的 parseOr。所有递归下降入口统一走深度计数。
func (p *parser) parseOrDepth(depth int) (exprNode, error) {
	if depth > maxExprDepth {
		return nil, fmt.Errorf("表达式嵌套深度超过上限 %d（防栈溢出）", maxExprDepth)
	}
	left, err := p.parseAndDepth(depth)
	if err != nil {
		return nil, err
	}
	for p.peek().kind == tokOr {
		op := p.next()
		right, err := p.parseAndDepth(depth)
		if err != nil {
			return nil, err
		}
		left = &binaryNode{op: op.text, left: left, right: right}
	}
	return left, nil
}

func (p *parser) parseAndDepth(depth int) (exprNode, error) {
	if depth > maxExprDepth {
		return nil, fmt.Errorf("表达式嵌套深度超过上限 %d（防栈溢出）", maxExprDepth)
	}
	left, err := p.parseNotDepth(depth)
	if err != nil {
		return nil, err
	}
	for p.peek().kind == tokAnd {
		op := p.next()
		right, err := p.parseNotDepth(depth)
		if err != nil {
			return nil, err
		}
		left = &binaryNode{op: op.text, left: left, right: right}
	}
	return left, nil
}

func (p *parser) parseNotDepth(depth int) (exprNode, error) {
	if depth > maxExprDepth {
		return nil, fmt.Errorf("表达式嵌套深度超过上限 %d（防栈溢出）", maxExprDepth)
	}
	if p.peek().kind == tokNot {
		p.next()
		node, err := p.parseNotDepth(depth)
		if err != nil {
			return nil, err
		}
		return &unaryNode{op: "NOT", node: node}, nil
	}
	return p.parseComparisonDepth(depth)
}

func (p *parser) parseComparisonDepth(depth int) (exprNode, error) {
	left, err := p.parseAdditiveDepth(depth)
	if err != nil {
		return nil, err
	}
	if p.peek().kind == tokOp {
		op := p.peek().text
		if op == "==" || op == "!=" || op == ">" || op == ">=" || op == "<" || op == "<=" {
			p.next()
			right, err := p.parseAdditiveDepth(depth)
			if err != nil {
				return nil, err
			}
			return &binaryNode{op: op, left: left, right: right}, nil
		}
	}
	return left, nil
}

func (p *parser) parseAdditiveDepth(depth int) (exprNode, error) {
	left, err := p.parseMultiplicativeDepth(depth)
	if err != nil {
		return nil, err
	}
	for p.peek().kind == tokOp && (p.peek().text == "+" || p.peek().text == "-") {
		op := p.next()
		right, err := p.parseMultiplicativeDepth(depth)
		if err != nil {
			return nil, err
		}
		left = &binaryNode{op: op.text, left: left, right: right}
	}
	return left, nil
}

func (p *parser) parseMultiplicativeDepth(depth int) (exprNode, error) {
	left, err := p.parsePrimaryDepth(depth)
	if err != nil {
		return nil, err
	}
	for p.peek().kind == tokOp && (p.peek().text == "*" || p.peek().text == "/") {
		op := p.next()
		right, err := p.parsePrimaryDepth(depth)
		if err != nil {
			return nil, err
		}
		left = &binaryNode{op: op.text, left: left, right: right}
	}
	return left, nil
}

func (p *parser) parsePrimaryDepth(depth int) (exprNode, error) {
	if depth > maxExprDepth {
		return nil, fmt.Errorf("表达式嵌套深度超过上限 %d（防栈溢出）", maxExprDepth)
	}
	t := p.next()
	switch t.kind {
	case tokNumber:
		return &numNode{value: t.value}, nil
	case tokIdent:
		if p.peek().kind == tokLParen {
			p.next() // 吃掉 (
			call := &callNode{name: t.text}
			if p.peek().kind != tokRParen {
				for {
					arg, err := p.parseOrDepth(depth + 1)
					if err != nil {
						return nil, err
					}
					call.args = append(call.args, arg)
					if p.peek().kind == tokComma {
						p.next()
						continue
					}
					break
				}
			}
			if p.peek().kind != tokRParen {
				return nil, fmt.Errorf("函数 %s 缺少右括号", t.text)
			}
			p.next()
			return call, nil
		}
		return &identNode{name: t.text}, nil
	case tokLParen:
		node, err := p.parseOrDepth(depth + 1)
		if err != nil {
			return nil, err
		}
		if p.peek().kind != tokRParen {
			return nil, fmt.Errorf("缺少右括号")
		}
		p.next()
		return node, nil
	}
	return nil, fmt.Errorf("意外的 token %q", t.text)
}

func (p *parser) peek() token { return p.toks[p.pos] }
func (p *parser) next() token { t := p.toks[p.pos]; p.pos++; return t }

// ---------------------------------------------------------------------
// 求值
// ---------------------------------------------------------------------

// evalNum 数值求值。
func evalNum(node exprNode, ctx *EvalContext) (float64, error) {
	switch n := node.(type) {
	case *numNode:
		return n.value, nil
	case *identNode:
		v, ok := ctx.at(n.name)
		if !ok {
			return 0, fmt.Errorf("未知标识符 %q（未在指标或字段中定义）", n.name)
		}
		return v, nil
	case *binaryNode:
		switch n.op {
		case "+":
			l, err := evalNum(n.left, ctx)
			if err != nil {
				return 0, err
			}
			r, err := evalNum(n.right, ctx)
			if err != nil {
				return 0, err
			}
			return l + r, nil
		case "-":
			l, err := evalNum(n.left, ctx)
			if err != nil {
				return 0, err
			}
			r, err := evalNum(n.right, ctx)
			if err != nil {
				return 0, err
			}
			return l - r, nil
		case "*":
			l, err := evalNum(n.left, ctx)
			if err != nil {
				return 0, err
			}
			r, err := evalNum(n.right, ctx)
			if err != nil {
				return 0, err
			}
			return l * r, nil
		case "/":
			l, err := evalNum(n.left, ctx)
			if err != nil {
				return 0, err
			}
			r, err := evalNum(n.right, ctx)
			if err != nil {
				return 0, err
			}
			if r == 0 {
				return math.NaN(), nil
			}
			return l / r, nil
		}
		return 0, fmt.Errorf("运算符 %s 不适用于数值", n.op)
	case *unaryNode:
		if n.op == "-" {
			v, err := evalNum(n.node, ctx)
			if err != nil {
				return 0, err
			}
			return -v, nil
		}
		return 0, fmt.Errorf("运算符 %s 不适用于数值", n.op)
	case *callNode:
		return evalCallNum(n, ctx)
	}
	return 0, fmt.Errorf("表达式无法求值为数值")
}

// evalCallNum 数值型内置函数。
func evalCallNum(call *callNode, ctx *EvalContext) (float64, error) {
	name := strings.ToLower(call.name)
	switch name {
	case "abs":
		if len(call.args) != 1 {
			return 0, fmt.Errorf("abs 需要 1 个参数")
		}
		v, err := evalNum(call.args[0], ctx)
		if err != nil {
			return 0, err
		}
		return math.Abs(v), nil
	case "ref":
		if len(call.args) != 2 {
			return 0, fmt.Errorf("ref(id, n) 需要 2 个参数")
		}
		id, err := evalIdentName(call.args[0])
		if err != nil {
			return 0, err
		}
		n, err := evalIntArg(call.args[1], ctx)
		if err != nil {
			return 0, err
		}
		// 无未来函数硬约束：n<0（含算术构造的负数，如 0-5）时视为非法，返回 NaN
		if n < 0 {
			return math.NaN(), nil
		}
		s, ok := ctx.series(id)
		if !ok {
			return 0, fmt.Errorf("ref: 未知标识符 %q", id)
		}
		idx := ctx.At - n
		if idx < 0 || idx >= len(s) {
			return math.NaN(), nil
		}
		return s[idx], nil
	case "highest", "lowest":
		if len(call.args) != 2 {
			return 0, fmt.Errorf("%s(id, n) 需要 2 个参数", name)
		}
		id, err := evalIdentName(call.args[0])
		if err != nil {
			return 0, err
		}
		n, err := evalIntArg(call.args[1], ctx)
		if err != nil {
			return 0, err
		}
		// 无未来函数硬约束：n<1 视为非法（含算术构造的负数），返回 NaN
		if n < 1 {
			return math.NaN(), nil
		}
		s, ok := ctx.series(id)
		if !ok {
			return 0, fmt.Errorf("%s: 未知标识符 %q", name, id)
		}
		if n < 1 {
			return math.NaN(), nil
		}
		start := ctx.At - n + 1
		if start < 0 {
			start = 0
		}
		if start > ctx.At {
			return math.NaN(), nil
		}
		result := s[start]
		if name == "highest" {
			for i := start + 1; i <= ctx.At; i++ {
				if s[i] > result {
					result = s[i]
				}
			}
		} else {
			for i := start + 1; i <= ctx.At; i++ {
				if s[i] < result {
					result = s[i]
				}
			}
		}
		return result, nil
	}
	return 0, fmt.Errorf("未知函数 %q", call.name)
}

// evalIdentName 参数为标识符时取名称。
func evalIdentName(node exprNode) (string, error) {
	if n, ok := node.(*identNode); ok {
		return n.name, nil
	}
	return "", fmt.Errorf("函数参数需要标识符")
}

func evalIntArg(node exprNode, ctx *EvalContext) (int, error) {
	v, err := evalNum(node, ctx)
	if err != nil {
		return 0, err
	}
	return int(v), nil
}

// evalBool 布尔求值。
func evalBool(node exprNode, ctx *EvalContext) (bool, error) {
	switch n := node.(type) {
	case *binaryNode:
		switch n.op {
		case "AND":
			l, err := evalBool(n.left, ctx)
			if err != nil {
				return false, err
			}
			if !l {
				return false, nil
			}
			return evalBool(n.right, ctx)
		case "OR":
			l, err := evalBool(n.left, ctx)
			if err != nil {
				return false, err
			}
			if l {
				return true, nil
			}
			return evalBool(n.right, ctx)
		case "==", "!=", ">", ">=", "<", "<=":
			l, err := evalNum(n.left, ctx)
			if err != nil {
				return false, err
			}
			r, err := evalNum(n.right, ctx)
			if err != nil {
				return false, err
			}
			if math.IsNaN(l) || math.IsNaN(r) {
				return false, nil
			}
			switch n.op {
			case "==":
				return l == r, nil
			case "!=":
				return l != r, nil
			case ">":
				return l > r, nil
			case ">=":
				return l >= r, nil
			case "<":
				return l < r, nil
			case "<=":
				return l <= r, nil
			}
		case "+", "-", "*", "/":
			// 数值表达式被当作布尔时（非零即真）
			v, err := evalNum(n, ctx)
			if err != nil {
				return false, err
			}
			return !math.IsNaN(v) && v != 0, nil
		}
	case *unaryNode:
		if n.op == "NOT" {
			v, err := evalBool(n.node, ctx)
			if err != nil {
				return false, err
			}
			return !v, nil
		}
	case *numNode:
		return n.value != 0, nil
	case *identNode:
		v, ok := ctx.at(n.name)
		if !ok {
			return false, fmt.Errorf("未知标识符 %q", n.name)
		}
		return !math.IsNaN(v) && v != 0, nil
	case *callNode:
		return evalCallBool(n, ctx)
	}
	return false, fmt.Errorf("表达式无法求值为布尔")
}

// evalCallBool 布尔型内置函数（cross_up / cross_down）。
func evalCallBool(call *callNode, ctx *EvalContext) (bool, error) {
	name := strings.ToLower(call.name)
	switch name {
	case "cross_up", "cross_down":
		if len(call.args) != 2 {
			return false, fmt.Errorf("%s(a, b) 需要 2 个参数", name)
		}
		idA, err := evalIdentName(call.args[0])
		if err != nil {
			return false, err
		}
		idB, err := evalIdentName(call.args[1])
		if err != nil {
			return false, err
		}
		sa, okA := ctx.series(idA)
		sb, okB := ctx.series(idB)
		if !okA || !okB {
			return false, fmt.Errorf("%s: 未知标识符", name)
		}
		i := ctx.At
		if i <= 0 || i >= len(sa) || i >= len(sb) {
			return false, nil
		}
		curA, prevA, curB, prevB := sa[i], sa[i-1], sb[i], sb[i-1]
		if math.IsNaN(curA) || math.IsNaN(prevA) || math.IsNaN(curB) || math.IsNaN(prevB) {
			return false, nil
		}
		if name == "cross_up" {
			return prevA <= prevB && curA > curB, nil
		}
		return prevA >= prevB && curA < curB, nil
	}
	return false, fmt.Errorf("未知函数 %q", call.name)
}

// knownFunctions 内置函数及其参数个数（-1 表示可变）。
var knownFunctions = map[string][2]int{
	"cross_up":   {2, 2},
	"cross_down": {2, 2},
	"ref":        {2, 2},
	"highest":    {2, 2},
	"lowest":     {2, 2},
	"abs":        {1, 1},
}

// constIntValue 若节点为数值常量（含一元负号与括号包裹）则返回其整数值。
func constIntValue(node exprNode) (int, bool) {
	switch n := node.(type) {
	case *numNode:
		return int(n.value), true
	case *unaryNode:
		if n.op == "-" {
			v, ok := constIntValue(n.node)
			return -v, ok
		}
	case *binaryNode:
		if n.op == "-" {
			l, ok1 := constIntValue(n.left)
			r, ok2 := constIntValue(n.right)
			if ok1 && ok2 {
				return l - r, true
			}
		}
	}
	return 0, false
}

// validateAST 校验 AST：
//  1. 函数名与参数个数合法性（编译期拦截，避免运行期才报错）；
//  2. ref/highest/lowest 第二参数若为常量则必须 >= 0（防前视：算术可构造负数，如 ref(close, 0-5)）；
//  3. 收集全部标识符引用（供信号表达式标识符交叉校验，见 collectIdentifiers）。
func validateAST(node exprNode) error {
	switch n := node.(type) {
	case *binaryNode:
		if err := validateAST(n.left); err != nil {
			return err
		}
		return validateAST(n.right)
	case *unaryNode:
		return validateAST(n.node)
	case *callNode:
		name := strings.ToLower(n.name)
		bounds, ok := knownFunctions[name]
		if !ok {
			return fmt.Errorf("未知函数 %q（支持: cross_up/cross_down/ref/highest/lowest/abs）", n.name)
		}
		if len(n.args) < bounds[0] || len(n.args) > bounds[1] {
			return fmt.Errorf("函数 %s 需要 %d 个参数，实际 %d", n.name, bounds[0], len(n.args))
		}
		// 前视漏洞防护：偏移类函数第二参数若为编译期常量，必须 >= 0
		if name == "ref" || name == "highest" || name == "lowest" {
			if v, isConst := constIntValue(n.args[1]); isConst && v < 0 {
				return fmt.Errorf("函数 %s 的偏移参数 n 必须 >= 0（无未来函数约束），当前为 %d", n.name, v)
			}
		}
		for _, arg := range n.args {
			if err := validateAST(arg); err != nil {
				return err
			}
		}
	}
	return nil
}

// collectIdentifiers 收集表达式中引用的全部标识符（指标 id / 字段名）。
// 供保存策略时与 indicators 声明 + 字段白名单交叉校验（拦“指标名写错”）。
func collectIdentifiers(node exprNode, out map[string]bool) {
	switch n := node.(type) {
	case *identNode:
		out[n.name] = true
	case *binaryNode:
		collectIdentifiers(n.left, out)
		collectIdentifiers(n.right, out)
	case *unaryNode:
		collectIdentifiers(n.node, out)
	case *callNode:
		for _, arg := range n.args {
			collectIdentifiers(arg, out)
		}
	}
}

// compileExprAST 编译表达式：语法校验 + AST 静态校验（函数/参数/偏移常量/深度）。
func compileExprAST(src string) (exprNode, error) {
	node, err := parseExpr(src)
	if err != nil {
		return nil, fmt.Errorf("信号表达式语法错误: %w", err)
	}
	if err := validateAST(node); err != nil {
		return nil, fmt.Errorf("信号表达式错误: %w", err)
	}
	return node, nil
}

// CompileExpr 编译表达式（校验语法、函数、偏移常量与深度），返回可直接求值的闭包。
func CompileExpr(src string) (func(ctx *EvalContext) (bool, error), error) {
	node, err := compileExprAST(src)
	if err != nil {
		return nil, err
	}
	return func(ctx *EvalContext) (bool, error) {
		return evalBool(node, ctx)
	}, nil
}

// CompileExprIdentifiers 编译表达式并返回引用的标识符集合（供保存期交叉校验）。
func CompileExprIdentifiers(src string) (map[string]bool, error) {
	node, err := compileExprAST(src)
	if err != nil {
		return nil, err
	}
	ids := map[string]bool{}
	collectIdentifiers(node, ids)
	return ids, nil
}
