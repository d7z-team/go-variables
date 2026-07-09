package variables

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

type Expression struct {
	source string
	root   exprNode
	calls  []string
}

type Match struct {
	Path  Path
	Value Value
}

func ParseExpression(src string, opts ...ExpressionOption) (Expression, error) {
	return CompileExpression(src, opts...)
}

func CompileExpression(src string, opts ...CompileOption) (Expression, error) {
	cfg := compileOptions{}
	for _, opt := range opts {
		opt(&cfg)
	}
	parser, err := newExpressionParser(src)
	if err != nil {
		return Expression{}, err
	}
	node, err := parser.parse()
	if err != nil {
		return Expression{}, err
	}
	expr := Expression{source: src, root: node, calls: parser.calls}
	if err := checkExpression(expr, cfg); err != nil {
		return Expression{}, err
	}
	return expr, nil
}

func MustExpression(src string, opts ...ExpressionOption) Expression {
	expr, err := ParseExpression(src, opts...)
	if err != nil {
		panic(err)
	}
	return expr
}

func MustCompileExpression(src string, opts ...CompileOption) Expression {
	expr, err := CompileExpression(src, opts...)
	if err != nil {
		panic(err)
	}
	return expr
}

func (e Expression) String() string {
	return e.source
}

type tokenKind int

const (
	tokenEOF tokenKind = iota
	tokenIdent
	tokenNumber
	tokenString
	tokenPlus
	tokenMinus
	tokenStar
	tokenSlash
	tokenPercent
	tokenEqual
	tokenNotEqual
	tokenGreater
	tokenGreaterEqual
	tokenLess
	tokenLessEqual
	tokenAnd
	tokenOr
	tokenNot
	tokenIn
	tokenDot
	tokenOptionalDot
	tokenFilterStart
	tokenLParen
	tokenRParen
	tokenLBracket
	tokenRBracket
	tokenLBrace
	tokenRBrace
	tokenComma
	tokenColon
	tokenDollar
)

type token struct {
	kind tokenKind
	text string
	pos  int
}

func lexExpression(src string) ([]token, error) {
	var tokens []token
	for i := 0; i < len(src); {
		r := rune(src[i])
		if unicode.IsSpace(r) {
			i++
			continue
		}
		start := i
		switch {
		case isIdentStart(src[i]):
			i++
			for i < len(src) && isIdentPart(src[i]) {
				i++
			}
			text := src[start:i]
			switch text {
			case "in":
				tokens = append(tokens, token{kind: tokenIn, text: text, pos: start})
			default:
				tokens = append(tokens, token{kind: tokenIdent, text: text, pos: start})
			}
		case src[i] >= '0' && src[i] <= '9':
			i++
			for i < len(src) && src[i] >= '0' && src[i] <= '9' {
				i++
			}
			if i < len(src) && src[i] == '.' {
				i++
				for i < len(src) && src[i] >= '0' && src[i] <= '9' {
					i++
				}
			}
			if i < len(src) && (src[i] == 'e' || src[i] == 'E') {
				i++
				if i < len(src) && (src[i] == '+' || src[i] == '-') {
					i++
				}
				exponentStart := i
				for i < len(src) && src[i] >= '0' && src[i] <= '9' {
					i++
				}
				if exponentStart == i {
					return nil, fmt.Errorf("invalid exponent at byte %d", start)
				}
			}
			tokens = append(tokens, token{kind: tokenNumber, text: src[start:i], pos: start})
		case src[i] == '"' || src[i] == '\'':
			quote := src[i]
			i++
			escaped := false
			for i < len(src) {
				if escaped {
					escaped = false
					i++
					continue
				}
				if src[i] == '\\' {
					escaped = true
					i++
					continue
				}
				if src[i] == quote {
					i++
					tokens = append(tokens, token{kind: tokenString, text: src[start:i], pos: start})
					goto next
				}
				i++
			}
			return nil, fmt.Errorf("unterminated string at byte %d", start)
		default:
			switch src[i] {
			case '+':
				tokens = append(tokens, token{kind: tokenPlus, text: "+", pos: i})
				i++
			case '-':
				tokens = append(tokens, token{kind: tokenMinus, text: "-", pos: i})
				i++
			case '*':
				tokens = append(tokens, token{kind: tokenStar, text: "*", pos: i})
				i++
			case '/':
				tokens = append(tokens, token{kind: tokenSlash, text: "/", pos: i})
				i++
			case '%':
				tokens = append(tokens, token{kind: tokenPercent, text: "%", pos: i})
				i++
			case '=':
				if i+1 < len(src) && src[i+1] == '=' {
					tokens = append(tokens, token{kind: tokenEqual, text: "==", pos: i})
					i += 2
				} else {
					return nil, fmt.Errorf("unexpected '=' at byte %d", i)
				}
			case '!':
				if i+1 < len(src) && src[i+1] == '=' {
					tokens = append(tokens, token{kind: tokenNotEqual, text: "!=", pos: i})
					i += 2
				} else {
					tokens = append(tokens, token{kind: tokenNot, text: "!", pos: i})
					i++
				}
			case '>':
				if i+1 < len(src) && src[i+1] == '=' {
					tokens = append(tokens, token{kind: tokenGreaterEqual, text: ">=", pos: i})
					i += 2
				} else {
					tokens = append(tokens, token{kind: tokenGreater, text: ">", pos: i})
					i++
				}
			case '<':
				if i+1 < len(src) && src[i+1] == '=' {
					tokens = append(tokens, token{kind: tokenLessEqual, text: "<=", pos: i})
					i += 2
				} else {
					tokens = append(tokens, token{kind: tokenLess, text: "<", pos: i})
					i++
				}
			case '&':
				if i+1 < len(src) && src[i+1] == '&' {
					tokens = append(tokens, token{kind: tokenAnd, text: "&&", pos: i})
					i += 2
				} else {
					return nil, fmt.Errorf("unexpected '&' at byte %d", i)
				}
			case '|':
				if i+1 < len(src) && src[i+1] == '|' {
					tokens = append(tokens, token{kind: tokenOr, text: "||", pos: i})
					i += 2
				} else {
					return nil, fmt.Errorf("unexpected pipe at byte %d", i)
				}
			case '.':
				tokens = append(tokens, token{kind: tokenDot, text: ".", pos: i})
				i++
			case '?':
				if i+1 < len(src) && src[i+1] == '.' {
					tokens = append(tokens, token{kind: tokenOptionalDot, text: "?.", pos: i})
					i += 2
				} else if i+1 < len(src) && src[i+1] == '[' {
					tokens = append(tokens, token{kind: tokenFilterStart, text: "?[", pos: i})
					i += 2
				} else {
					return nil, fmt.Errorf("unexpected '?' at byte %d", i)
				}
			case '(':
				tokens = append(tokens, token{kind: tokenLParen, text: "(", pos: i})
				i++
			case ')':
				tokens = append(tokens, token{kind: tokenRParen, text: ")", pos: i})
				i++
			case '[':
				tokens = append(tokens, token{kind: tokenLBracket, text: "[", pos: i})
				i++
			case ']':
				tokens = append(tokens, token{kind: tokenRBracket, text: "]", pos: i})
				i++
			case '{':
				tokens = append(tokens, token{kind: tokenLBrace, text: "{", pos: i})
				i++
			case '}':
				tokens = append(tokens, token{kind: tokenRBrace, text: "}", pos: i})
				i++
			case ',':
				tokens = append(tokens, token{kind: tokenComma, text: ",", pos: i})
				i++
			case ':':
				tokens = append(tokens, token{kind: tokenColon, text: ":", pos: i})
				i++
			case '$':
				tokens = append(tokens, token{kind: tokenDollar, text: "$", pos: i})
				i++
			default:
				return nil, fmt.Errorf("unexpected %q at byte %d", src[i], i)
			}
		}
	next:
	}
	tokens = append(tokens, token{kind: tokenEOF, pos: len(src)})
	return tokens, nil
}

func isIdentStart(b byte) bool {
	return b == '_' || b >= 'A' && b <= 'Z' || b >= 'a' && b <= 'z'
}

func isIdentPart(b byte) bool {
	return isIdentStart(b) || b >= '0' && b <= '9'
}

type expressionParser struct {
	tokens []token
	pos    int
	calls  []string
}

func newExpressionParser(src string) (*expressionParser, error) {
	tokens, err := lexExpression(src)
	if err != nil {
		return nil, err
	}
	return &expressionParser{tokens: tokens}, nil
}

func (p *expressionParser) parse() (exprNode, error) {
	node, err := p.parseExpression(1)
	if err != nil {
		return nil, err
	}
	if p.peek().kind != tokenEOF {
		return nil, fmt.Errorf("unexpected %q at byte %d", p.peek().text, p.peek().pos)
	}
	return node, nil
}

func (p *expressionParser) parseExpression(minPrec int) (exprNode, error) {
	left, err := p.parseUnary()
	if err != nil {
		return nil, err
	}
	for {
		op := p.peek()
		prec := binaryPrecedence(op.kind)
		if prec < minPrec {
			return left, nil
		}
		p.next()
		right, err := p.parseExpression(prec + 1)
		if err != nil {
			return nil, err
		}
		left = binaryNode{op: op.kind, left: left, right: right, pos: op.pos}
	}
}

func (p *expressionParser) parseUnary() (exprNode, error) {
	switch p.peek().kind {
	case tokenNot, tokenMinus:
		op := p.next()
		node, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return unaryNode{op: op.kind, child: node, pos: op.pos}, nil
	default:
		return p.parsePostfix()
	}
}

func (p *expressionParser) parsePostfix() (exprNode, error) {
	node, err := p.parsePrimary()
	if err != nil {
		return nil, err
	}
	for {
		switch p.peek().kind {
		case tokenLParen:
			ident, ok := node.(identifierNode)
			if !ok {
				return nil, fmt.Errorf("only named functions can be called at byte %d", p.peek().pos)
			}
			if ident.name == "query" {
				return nil, fmt.Errorf("query function is not supported")
			}
			args, err := p.parseArguments()
			if err != nil {
				return nil, err
			}
			p.calls = append(p.calls, ident.name)
			node = callNode{name: ident.name, args: args, pos: ident.pos}
		case tokenDot, tokenOptionalDot:
			optional := p.next().kind == tokenOptionalDot
			name := p.expect(tokenIdent)
			if name.kind != tokenIdent {
				return nil, fmt.Errorf("expected field name at byte %d", p.peek().pos)
			}
			if p.peek().kind == tokenLParen {
				if optional {
					return nil, fmt.Errorf("optional method calls are not supported at byte %d", name.pos)
				}
				if name.text == "query" {
					return nil, fmt.Errorf("query function is not supported")
				}
				args, err := p.parseArguments()
				if err != nil {
					return nil, err
				}
				p.calls = append(p.calls, name.text)
				node = methodCallNode{receiver: node, name: name.text, args: args, pos: name.pos}
				continue
			}
			node = memberNode{receiver: node, name: name.text, optional: optional, pos: name.pos}
		case tokenLBracket:
			start := p.next()
			index, err := p.parseExpression(1)
			if err != nil {
				return nil, err
			}
			if p.expect(tokenRBracket).kind != tokenRBracket {
				return nil, fmt.Errorf("expected ']' at byte %d", p.peek().pos)
			}
			node = indexNode{receiver: node, index: index, pos: start.pos}
		case tokenFilterStart:
			start := p.next()
			predicate, err := p.parseExpression(1)
			if err != nil {
				return nil, err
			}
			if p.expect(tokenRBracket).kind != tokenRBracket {
				return nil, fmt.Errorf("expected ']' at byte %d", p.peek().pos)
			}
			node = filterNode{receiver: node, predicate: predicate, pos: start.pos}
		default:
			return node, nil
		}
	}
}

func (p *expressionParser) parsePrimary() (exprNode, error) {
	tok := p.next()
	switch tok.kind {
	case tokenIdent:
		switch tok.text {
		case "true":
			return literalNode{value: true, pos: tok.pos}, nil
		case "false":
			return literalNode{value: false, pos: tok.pos}, nil
		case "null":
			return literalNode{value: nil, pos: tok.pos}, nil
		default:
			return identifierNode{name: tok.text, pos: tok.pos}, nil
		}
	case tokenDollar:
		return rootNode{pos: tok.pos}, nil
	case tokenNumber:
		value, err := parseJSONNumber(json.Number(tok.text))
		if err != nil {
			return nil, err
		}
		return literalNode{value: value, pos: tok.pos}, nil
	case tokenString:
		value, err := unquoteExpressionString(tok.text)
		if err != nil {
			return nil, err
		}
		return literalNode{value: value, pos: tok.pos}, nil
	case tokenLParen:
		node, err := p.parseExpression(1)
		if err != nil {
			return nil, err
		}
		if p.expect(tokenRParen).kind != tokenRParen {
			return nil, fmt.Errorf("expected ')' at byte %d", p.peek().pos)
		}
		return node, nil
	case tokenLBracket:
		return p.parseArrayLiteral(tok.pos)
	case tokenLBrace:
		return p.parseObjectLiteral(tok.pos)
	default:
		return nil, fmt.Errorf("unexpected %q at byte %d", tok.text, tok.pos)
	}
}

func unquoteExpressionString(src string) (string, error) {
	if strings.HasPrefix(src, `"`) {
		return strconv.Unquote(src)
	}
	var out strings.Builder
	for i := 1; i < len(src)-1; i++ {
		if src[i] != '\\' {
			out.WriteByte(src[i])
			continue
		}
		i++
		if i >= len(src)-1 {
			return "", fmt.Errorf("dangling escape in string")
		}
		switch src[i] {
		case 'n':
			out.WriteByte('\n')
		case 'r':
			out.WriteByte('\r')
		case 't':
			out.WriteByte('\t')
		default:
			out.WriteByte(src[i])
		}
	}
	return out.String(), nil
}

func (p *expressionParser) parseArrayLiteral(pos int) (exprNode, error) {
	var items []exprNode
	if p.peek().kind == tokenRBracket {
		p.next()
		return arrayNode{pos: pos}, nil
	}
	for {
		item, err := p.parseExpression(1)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
		if p.peek().kind != tokenComma {
			break
		}
		p.next()
		if p.peek().kind == tokenRBracket {
			return nil, fmt.Errorf("trailing comma in array at byte %d", p.peek().pos)
		}
	}
	if p.expect(tokenRBracket).kind != tokenRBracket {
		return nil, fmt.Errorf("expected ']' at byte %d", p.peek().pos)
	}
	return arrayNode{items: items, pos: pos}, nil
}

func (p *expressionParser) parseObjectLiteral(pos int) (exprNode, error) {
	object := objectNode{items: map[string]exprNode{}, pos: pos}
	if p.peek().kind == tokenRBrace {
		p.next()
		return object, nil
	}
	for {
		keyToken := p.next()
		var key string
		switch keyToken.kind {
		case tokenIdent:
			key = keyToken.text
		case tokenString:
			value, err := unquoteExpressionString(keyToken.text)
			if err != nil {
				return nil, err
			}
			key = value
		default:
			return nil, fmt.Errorf("expected object key at byte %d", keyToken.pos)
		}
		if p.expect(tokenColon).kind != tokenColon {
			return nil, fmt.Errorf("expected ':' at byte %d", p.peek().pos)
		}
		if _, ok := object.items[key]; ok {
			return nil, fmt.Errorf("duplicate object key %q at byte %d", key, keyToken.pos)
		}
		value, err := p.parseExpression(1)
		if err != nil {
			return nil, err
		}
		object.keys = append(object.keys, key)
		object.items[key] = value
		if p.peek().kind != tokenComma {
			break
		}
		p.next()
		if p.peek().kind == tokenRBrace {
			return nil, fmt.Errorf("trailing comma in object at byte %d", p.peek().pos)
		}
	}
	if p.expect(tokenRBrace).kind != tokenRBrace {
		return nil, fmt.Errorf("expected '}' at byte %d", p.peek().pos)
	}
	return object, nil
}

func (p *expressionParser) parseArguments() ([]exprNode, error) {
	p.next()
	var args []exprNode
	if p.peek().kind == tokenRParen {
		p.next()
		return args, nil
	}
	for {
		arg, err := p.parseExpression(1)
		if err != nil {
			return nil, err
		}
		args = append(args, arg)
		if p.peek().kind != tokenComma {
			break
		}
		p.next()
		if p.peek().kind == tokenRParen {
			return nil, fmt.Errorf("trailing comma in call at byte %d", p.peek().pos)
		}
	}
	if p.expect(tokenRParen).kind != tokenRParen {
		return nil, fmt.Errorf("expected ')' at byte %d", p.peek().pos)
	}
	return args, nil
}

func (p *expressionParser) peek() token {
	return p.tokens[p.pos]
}

func (p *expressionParser) next() token {
	tok := p.tokens[p.pos]
	p.pos++
	return tok
}

func (p *expressionParser) expect(kind tokenKind) token {
	if p.peek().kind != kind {
		return token{}
	}
	return p.next()
}

func binaryPrecedence(kind tokenKind) int {
	switch kind {
	case tokenOr:
		return 1
	case tokenAnd:
		return 2
	case tokenEqual, tokenNotEqual, tokenGreater, tokenGreaterEqual, tokenLess, tokenLessEqual, tokenIn:
		return 3
	case tokenPlus, tokenMinus:
		return 4
	case tokenStar, tokenSlash, tokenPercent:
		return 5
	default:
		return 0
	}
}
