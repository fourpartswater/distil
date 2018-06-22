package result

//go:generate peg -inline ./api/compute/result/complex_field.peg

import (
	"fmt"
	"math"
	"sort"
	"strconv"
)

const endSymbol rune = 1114112

/* The rule types inferred from the grammar are below. */
type pegRule uint8

const (
	ruleUnknown pegRule = iota
	ruleComplexField
	rulearray
	ruleitem
	rulestring
	ruledquote_string
	rulesquote_string
	rulevalue
	rulews
	rulecomma
	rulelf
	rulecr
	ruleescdquote
	ruleescsquote
	rulesquote
	ruleobracket
	rulecbracket
	rulenumber
	rulenegative
	ruledecimal_point
	ruletextdata
	ruleAction0
	ruleAction1
	rulePegText
	ruleAction2
	ruleAction3
	ruleAction4
)

var rul3s = [...]string{
	"Unknown",
	"ComplexField",
	"array",
	"item",
	"string",
	"dquote_string",
	"squote_string",
	"value",
	"ws",
	"comma",
	"lf",
	"cr",
	"escdquote",
	"escsquote",
	"squote",
	"obracket",
	"cbracket",
	"number",
	"negative",
	"decimal_point",
	"textdata",
	"Action0",
	"Action1",
	"PegText",
	"Action2",
	"Action3",
	"Action4",
}

type token32 struct {
	pegRule
	begin, end uint32
}

func (t *token32) String() string {
	return fmt.Sprintf("\x1B[34m%v\x1B[m %v %v", rul3s[t.pegRule], t.begin, t.end)
}

type node32 struct {
	token32
	up, next *node32
}

func (node *node32) print(pretty bool, buffer string) {
	var print func(node *node32, depth int)
	print = func(node *node32, depth int) {
		for node != nil {
			for c := 0; c < depth; c++ {
				fmt.Printf(" ")
			}
			rule := rul3s[node.pegRule]
			quote := strconv.Quote(string(([]rune(buffer)[node.begin:node.end])))
			if !pretty {
				fmt.Printf("%v %v\n", rule, quote)
			} else {
				fmt.Printf("\x1B[34m%v\x1B[m %v\n", rule, quote)
			}
			if node.up != nil {
				print(node.up, depth+1)
			}
			node = node.next
		}
	}
	print(node, 0)
}

func (node *node32) Print(buffer string) {
	node.print(false, buffer)
}

func (node *node32) PrettyPrint(buffer string) {
	node.print(true, buffer)
}

type tokens32 struct {
	tree []token32
}

func (t *tokens32) Trim(length uint32) {
	t.tree = t.tree[:length]
}

func (t *tokens32) Print() {
	for _, token := range t.tree {
		fmt.Println(token.String())
	}
}

func (t *tokens32) AST() *node32 {
	type element struct {
		node *node32
		down *element
	}
	tokens := t.Tokens()
	var stack *element
	for _, token := range tokens {
		if token.begin == token.end {
			continue
		}
		node := &node32{token32: token}
		for stack != nil && stack.node.begin >= token.begin && stack.node.end <= token.end {
			stack.node.next = node.up
			node.up = stack.node
			stack = stack.down
		}
		stack = &element{node: node, down: stack}
	}
	if stack != nil {
		return stack.node
	}
	return nil
}

func (t *tokens32) PrintSyntaxTree(buffer string) {
	t.AST().Print(buffer)
}

func (t *tokens32) PrettyPrintSyntaxTree(buffer string) {
	t.AST().PrettyPrint(buffer)
}

func (t *tokens32) Add(rule pegRule, begin, end, index uint32) {
	if tree := t.tree; int(index) >= len(tree) {
		expanded := make([]token32, 2*len(tree))
		copy(expanded, tree)
		t.tree = expanded
	}
	t.tree[index] = token32{
		pegRule: rule,
		begin:   begin,
		end:     end,
	}
}

func (t *tokens32) Tokens() []token32 {
	return t.tree
}

type ComplexField struct {
	arrayElements

	Buffer string
	buffer []rune
	rules  [27]func() bool
	parse  func(rule ...int) error
	reset  func()
	Pretty bool
	tokens32
}

func (p *ComplexField) Parse(rule ...int) error {
	return p.parse(rule...)
}

func (p *ComplexField) Reset() {
	p.reset()
}

type textPosition struct {
	line, symbol int
}

type textPositionMap map[int]textPosition

func translatePositions(buffer []rune, positions []int) textPositionMap {
	length, translations, j, line, symbol := len(positions), make(textPositionMap, len(positions)), 0, 1, 0
	sort.Ints(positions)

search:
	for i, c := range buffer {
		if c == '\n' {
			line, symbol = line+1, 0
		} else {
			symbol++
		}
		if i == positions[j] {
			translations[positions[j]] = textPosition{line, symbol}
			for j++; j < length; j++ {
				if i != positions[j] {
					continue search
				}
			}
			break search
		}
	}

	return translations
}

type parseError struct {
	p   *ComplexField
	max token32
}

func (e *parseError) Error() string {
	tokens, error := []token32{e.max}, "\n"
	positions, p := make([]int, 2*len(tokens)), 0
	for _, token := range tokens {
		positions[p], p = int(token.begin), p+1
		positions[p], p = int(token.end), p+1
	}
	translations := translatePositions(e.p.buffer, positions)
	format := "parse error near %v (line %v symbol %v - line %v symbol %v):\n%v\n"
	if e.p.Pretty {
		format = "parse error near \x1B[34m%v\x1B[m (line %v symbol %v - line %v symbol %v):\n%v\n"
	}
	for _, token := range tokens {
		begin, end := int(token.begin), int(token.end)
		error += fmt.Sprintf(format,
			rul3s[token.pegRule],
			translations[begin].line, translations[begin].symbol,
			translations[end].line, translations[end].symbol,
			strconv.Quote(string(e.p.buffer[begin:end])))
	}

	return error
}

func (p *ComplexField) PrintSyntaxTree() {
	if p.Pretty {
		p.tokens32.PrettyPrintSyntaxTree(p.Buffer)
	} else {
		p.tokens32.PrintSyntaxTree(p.Buffer)
	}
}

func (p *ComplexField) Execute() {
	buffer, _buffer, text, begin, end := p.Buffer, p.buffer, "", 0, 0
	for _, token := range p.Tokens() {
		switch token.pegRule {

		case rulePegText:
			begin, end = int(token.begin), int(token.end)
			text = string(_buffer[begin:end])

		case ruleAction0:
			p.pushArray()
		case ruleAction1:
			p.popArray()
		case ruleAction2:
			p.addElement(buffer[begin:end])
		case ruleAction3:
			p.addElement(buffer[begin:end])
		case ruleAction4:
			p.addElement(buffer[begin:end])

		}
	}
	_, _, _, _, _ = buffer, _buffer, text, begin, end
}

func (p *ComplexField) Init() {
	var (
		max                  token32
		position, tokenIndex uint32
		buffer               []rune
	)
	p.reset = func() {
		max = token32{}
		position, tokenIndex = 0, 0

		p.buffer = []rune(p.Buffer)
		if len(p.buffer) == 0 || p.buffer[len(p.buffer)-1] != endSymbol {
			p.buffer = append(p.buffer, endSymbol)
		}
		buffer = p.buffer
	}
	p.reset()

	_rules := p.rules
	tree := tokens32{tree: make([]token32, math.MaxInt16)}
	p.parse = func(rule ...int) error {
		r := 1
		if len(rule) > 0 {
			r = rule[0]
		}
		matches := p.rules[r]()
		p.tokens32 = tree
		if matches {
			p.Trim(tokenIndex)
			return nil
		}
		return &parseError{p, max}
	}

	add := func(rule pegRule, begin uint32) {
		tree.Add(rule, begin, position, tokenIndex)
		tokenIndex++
		if begin != position && position > max.end {
			max = token32{rule, begin, position}
		}
	}

	matchDot := func() bool {
		if buffer[position] != endSymbol {
			position++
			return true
		}
		return false
	}

	/*matchChar := func(c byte) bool {
		if buffer[position] == c {
			position++
			return true
		}
		return false
	}*/

	/*matchRange := func(lower byte, upper byte) bool {
		if c := buffer[position]; c >= lower && c <= upper {
			position++
			return true
		}
		return false
	}*/

	_rules = [...]func() bool{
		nil,
		/* 0 ComplexField <- <(array !.)> */
		func() bool {
			position0, tokenIndex0 := position, tokenIndex
			{
				position1 := position
				if !_rules[rulearray]() {
					goto l0
				}
				{
					position2, tokenIndex2 := position, tokenIndex
					if !matchDot() {
						goto l2
					}
					goto l0
				l2:
					position, tokenIndex = position2, tokenIndex2
				}
				add(ruleComplexField, position1)
			}
			return true
		l0:
			position, tokenIndex = position0, tokenIndex0
			return false
		},
		/* 1 array <- <(ws* obracket Action0 ws* (item ws* (comma ws* item ws*)*)? cbracket Action1)> */
		func() bool {
			position3, tokenIndex3 := position, tokenIndex
			{
				position4 := position
			l5:
				{
					position6, tokenIndex6 := position, tokenIndex
					if !_rules[rulews]() {
						goto l6
					}
					goto l5
				l6:
					position, tokenIndex = position6, tokenIndex6
				}
				if !_rules[ruleobracket]() {
					goto l3
				}
				{
					add(ruleAction0, position)
				}
			l8:
				{
					position9, tokenIndex9 := position, tokenIndex
					if !_rules[rulews]() {
						goto l9
					}
					goto l8
				l9:
					position, tokenIndex = position9, tokenIndex9
				}
				{
					position10, tokenIndex10 := position, tokenIndex
					if !_rules[ruleitem]() {
						goto l10
					}
				l12:
					{
						position13, tokenIndex13 := position, tokenIndex
						if !_rules[rulews]() {
							goto l13
						}
						goto l12
					l13:
						position, tokenIndex = position13, tokenIndex13
					}
				l14:
					{
						position15, tokenIndex15 := position, tokenIndex
						if !_rules[rulecomma]() {
							goto l15
						}
					l16:
						{
							position17, tokenIndex17 := position, tokenIndex
							if !_rules[rulews]() {
								goto l17
							}
							goto l16
						l17:
							position, tokenIndex = position17, tokenIndex17
						}
						if !_rules[ruleitem]() {
							goto l15
						}
					l18:
						{
							position19, tokenIndex19 := position, tokenIndex
							if !_rules[rulews]() {
								goto l19
							}
							goto l18
						l19:
							position, tokenIndex = position19, tokenIndex19
						}
						goto l14
					l15:
						position, tokenIndex = position15, tokenIndex15
					}
					goto l11
				l10:
					position, tokenIndex = position10, tokenIndex10
				}
			l11:
				if !_rules[rulecbracket]() {
					goto l3
				}
				{
					add(ruleAction1, position)
				}
				add(rulearray, position4)
			}
			return true
		l3:
			position, tokenIndex = position3, tokenIndex3
			return false
		},
		/* 2 item <- <(array / string / (<value*> Action2))> */
		func() bool {
			{
				position22 := position
				{
					position23, tokenIndex23 := position, tokenIndex
					if !_rules[rulearray]() {
						goto l24
					}
					goto l23
				l24:
					position, tokenIndex = position23, tokenIndex23
					{
						position26 := position
						{
							position27, tokenIndex27 := position, tokenIndex
							{
								position29 := position
								if !_rules[ruleescdquote]() {
									goto l28
								}
								{
									position30 := position
								l31:
									{
										position32, tokenIndex32 := position, tokenIndex
										{
											position33, tokenIndex33 := position, tokenIndex
											if !_rules[ruletextdata]() {
												goto l34
											}
											goto l33
										l34:
											position, tokenIndex = position33, tokenIndex33
											if !_rules[rulesquote]() {
												goto l35
											}
											goto l33
										l35:
											position, tokenIndex = position33, tokenIndex33
											if !_rules[rulelf]() {
												goto l36
											}
											goto l33
										l36:
											position, tokenIndex = position33, tokenIndex33
											if !_rules[rulecr]() {
												goto l37
											}
											goto l33
										l37:
											position, tokenIndex = position33, tokenIndex33
											if !_rules[ruleobracket]() {
												goto l38
											}
											goto l33
										l38:
											position, tokenIndex = position33, tokenIndex33
											if !_rules[rulecbracket]() {
												goto l39
											}
											goto l33
										l39:
											position, tokenIndex = position33, tokenIndex33
											if !_rules[rulecomma]() {
												goto l32
											}
										}
									l33:
										goto l31
									l32:
										position, tokenIndex = position32, tokenIndex32
									}
									add(rulePegText, position30)
								}
								if !_rules[ruleescdquote]() {
									goto l28
								}
								{
									add(ruleAction3, position)
								}
								add(ruledquote_string, position29)
							}
							goto l27
						l28:
							position, tokenIndex = position27, tokenIndex27
							{
								position41 := position
								if !_rules[rulesquote]() {
									goto l25
								}
								{
									position42 := position
								l43:
									{
										position44, tokenIndex44 := position, tokenIndex
										{
											position45, tokenIndex45 := position, tokenIndex
											{
												position47 := position
												if buffer[position] != rune('\\') {
													goto l46
												}
												position++
												if buffer[position] != rune('\'') {
													goto l46
												}
												position++
												add(ruleescsquote, position47)
											}
											goto l45
										l46:
											position, tokenIndex = position45, tokenIndex45
											if !_rules[ruleescdquote]() {
												goto l48
											}
											goto l45
										l48:
											position, tokenIndex = position45, tokenIndex45
											if !_rules[ruletextdata]() {
												goto l49
											}
											goto l45
										l49:
											position, tokenIndex = position45, tokenIndex45
											if !_rules[rulelf]() {
												goto l50
											}
											goto l45
										l50:
											position, tokenIndex = position45, tokenIndex45
											if !_rules[rulecr]() {
												goto l51
											}
											goto l45
										l51:
											position, tokenIndex = position45, tokenIndex45
											if !_rules[ruleobracket]() {
												goto l52
											}
											goto l45
										l52:
											position, tokenIndex = position45, tokenIndex45
											if !_rules[rulecbracket]() {
												goto l44
											}
										}
									l45:
										goto l43
									l44:
										position, tokenIndex = position44, tokenIndex44
									}
									add(rulePegText, position42)
								}
								if !_rules[rulesquote]() {
									goto l25
								}
								{
									add(ruleAction4, position)
								}
								add(rulesquote_string, position41)
							}
						}
					l27:
						add(rulestring, position26)
					}
					goto l23
				l25:
					position, tokenIndex = position23, tokenIndex23
					{
						position54 := position
					l55:
						{
							position56, tokenIndex56 := position, tokenIndex
							{
								position57 := position
								{
									position58, tokenIndex58 := position, tokenIndex
									{
										position60 := position
										if buffer[position] != rune('-') {
											goto l58
										}
										position++
										add(rulenegative, position60)
									}
									goto l59
								l58:
									position, tokenIndex = position58, tokenIndex58
								}
							l59:
								if !_rules[rulenumber]() {
									goto l56
								}
							l61:
								{
									position62, tokenIndex62 := position, tokenIndex
									if !_rules[rulenumber]() {
										goto l62
									}
									goto l61
								l62:
									position, tokenIndex = position62, tokenIndex62
								}
								{
									position63, tokenIndex63 := position, tokenIndex
									{
										position65 := position
										if buffer[position] != rune('.') {
											goto l63
										}
										position++
										add(ruledecimal_point, position65)
									}
									if !_rules[rulenumber]() {
										goto l63
									}
								l66:
									{
										position67, tokenIndex67 := position, tokenIndex
										if !_rules[rulenumber]() {
											goto l67
										}
										goto l66
									l67:
										position, tokenIndex = position67, tokenIndex67
									}
									goto l64
								l63:
									position, tokenIndex = position63, tokenIndex63
								}
							l64:
								add(rulevalue, position57)
							}
							goto l55
						l56:
							position, tokenIndex = position56, tokenIndex56
						}
						add(rulePegText, position54)
					}
					{
						add(ruleAction2, position)
					}
				}
			l23:
				add(ruleitem, position22)
			}
			return true
		},
		/* 3 string <- <(dquote_string / squote_string)> */
		nil,
		/* 4 dquote_string <- <(escdquote <(textdata / squote / lf / cr / obracket / cbracket / comma)*> escdquote Action3)> */
		nil,
		/* 5 squote_string <- <(squote <(escsquote / escdquote / textdata / lf / cr / obracket / cbracket)*> squote Action4)> */
		nil,
		/* 6 value <- <(negative? number+ (decimal_point number+)?)> */
		nil,
		/* 7 ws <- <' '> */
		func() bool {
			position73, tokenIndex73 := position, tokenIndex
			{
				position74 := position
				if buffer[position] != rune(' ') {
					goto l73
				}
				position++
				add(rulews, position74)
			}
			return true
		l73:
			position, tokenIndex = position73, tokenIndex73
			return false
		},
		/* 8 comma <- <','> */
		func() bool {
			position75, tokenIndex75 := position, tokenIndex
			{
				position76 := position
				if buffer[position] != rune(',') {
					goto l75
				}
				position++
				add(rulecomma, position76)
			}
			return true
		l75:
			position, tokenIndex = position75, tokenIndex75
			return false
		},
		/* 9 lf <- <'\n'> */
		func() bool {
			position77, tokenIndex77 := position, tokenIndex
			{
				position78 := position
				if buffer[position] != rune('\n') {
					goto l77
				}
				position++
				add(rulelf, position78)
			}
			return true
		l77:
			position, tokenIndex = position77, tokenIndex77
			return false
		},
		/* 10 cr <- <'\r'> */
		func() bool {
			position79, tokenIndex79 := position, tokenIndex
			{
				position80 := position
				if buffer[position] != rune('\r') {
					goto l79
				}
				position++
				add(rulecr, position80)
			}
			return true
		l79:
			position, tokenIndex = position79, tokenIndex79
			return false
		},
		/* 11 escdquote <- <'"'> */
		func() bool {
			position81, tokenIndex81 := position, tokenIndex
			{
				position82 := position
				if buffer[position] != rune('"') {
					goto l81
				}
				position++
				add(ruleescdquote, position82)
			}
			return true
		l81:
			position, tokenIndex = position81, tokenIndex81
			return false
		},
		/* 12 escsquote <- <('\\' '\'')> */
		nil,
		/* 13 squote <- <'\''> */
		func() bool {
			position84, tokenIndex84 := position, tokenIndex
			{
				position85 := position
				if buffer[position] != rune('\'') {
					goto l84
				}
				position++
				add(rulesquote, position85)
			}
			return true
		l84:
			position, tokenIndex = position84, tokenIndex84
			return false
		},
		/* 14 obracket <- <'['> */
		func() bool {
			position86, tokenIndex86 := position, tokenIndex
			{
				position87 := position
				if buffer[position] != rune('[') {
					goto l86
				}
				position++
				add(ruleobracket, position87)
			}
			return true
		l86:
			position, tokenIndex = position86, tokenIndex86
			return false
		},
		/* 15 cbracket <- <']'> */
		func() bool {
			position88, tokenIndex88 := position, tokenIndex
			{
				position89 := position
				if buffer[position] != rune(']') {
					goto l88
				}
				position++
				add(rulecbracket, position89)
			}
			return true
		l88:
			position, tokenIndex = position88, tokenIndex88
			return false
		},
		/* 16 number <- <([a-z] / [A-Z] / [0-9])> */
		func() bool {
			position90, tokenIndex90 := position, tokenIndex
			{
				position91 := position
				{
					position92, tokenIndex92 := position, tokenIndex
					if c := buffer[position]; c < rune('a') || c > rune('z') {
						goto l93
					}
					position++
					goto l92
				l93:
					position, tokenIndex = position92, tokenIndex92
					if c := buffer[position]; c < rune('A') || c > rune('Z') {
						goto l94
					}
					position++
					goto l92
				l94:
					position, tokenIndex = position92, tokenIndex92
					if c := buffer[position]; c < rune('0') || c > rune('9') {
						goto l90
					}
					position++
				}
			l92:
				add(rulenumber, position91)
			}
			return true
		l90:
			position, tokenIndex = position90, tokenIndex90
			return false
		},
		/* 17 negative <- <'-'> */
		nil,
		/* 18 decimal_point <- <'.'> */
		nil,
		/* 19 textdata <- <([a-z] / [A-Z] / [0-9] / ' ' / '!' / '#' / '$' / '&' / '%' / '(' / ')' / '*' / '+' / '-' / '.' / '/' / ':' / ';' / [<->] / '?' / '\\' / '^' / '_' / '`' / '{' / '|' / '}' / '~')> */
		func() bool {
			position97, tokenIndex97 := position, tokenIndex
			{
				position98 := position
				{
					position99, tokenIndex99 := position, tokenIndex
					if c := buffer[position]; c < rune('a') || c > rune('z') {
						goto l100
					}
					position++
					goto l99
				l100:
					position, tokenIndex = position99, tokenIndex99
					if c := buffer[position]; c < rune('A') || c > rune('Z') {
						goto l101
					}
					position++
					goto l99
				l101:
					position, tokenIndex = position99, tokenIndex99
					if c := buffer[position]; c < rune('0') || c > rune('9') {
						goto l102
					}
					position++
					goto l99
				l102:
					position, tokenIndex = position99, tokenIndex99
					if buffer[position] != rune(' ') {
						goto l103
					}
					position++
					goto l99
				l103:
					position, tokenIndex = position99, tokenIndex99
					if buffer[position] != rune('!') {
						goto l104
					}
					position++
					goto l99
				l104:
					position, tokenIndex = position99, tokenIndex99
					if buffer[position] != rune('#') {
						goto l105
					}
					position++
					goto l99
				l105:
					position, tokenIndex = position99, tokenIndex99
					if buffer[position] != rune('$') {
						goto l106
					}
					position++
					goto l99
				l106:
					position, tokenIndex = position99, tokenIndex99
					if buffer[position] != rune('&') {
						goto l107
					}
					position++
					goto l99
				l107:
					position, tokenIndex = position99, tokenIndex99
					if buffer[position] != rune('%') {
						goto l108
					}
					position++
					goto l99
				l108:
					position, tokenIndex = position99, tokenIndex99
					if buffer[position] != rune('(') {
						goto l109
					}
					position++
					goto l99
				l109:
					position, tokenIndex = position99, tokenIndex99
					if buffer[position] != rune(')') {
						goto l110
					}
					position++
					goto l99
				l110:
					position, tokenIndex = position99, tokenIndex99
					if buffer[position] != rune('*') {
						goto l111
					}
					position++
					goto l99
				l111:
					position, tokenIndex = position99, tokenIndex99
					if buffer[position] != rune('+') {
						goto l112
					}
					position++
					goto l99
				l112:
					position, tokenIndex = position99, tokenIndex99
					if buffer[position] != rune('-') {
						goto l113
					}
					position++
					goto l99
				l113:
					position, tokenIndex = position99, tokenIndex99
					if buffer[position] != rune('.') {
						goto l114
					}
					position++
					goto l99
				l114:
					position, tokenIndex = position99, tokenIndex99
					if buffer[position] != rune('/') {
						goto l115
					}
					position++
					goto l99
				l115:
					position, tokenIndex = position99, tokenIndex99
					if buffer[position] != rune(':') {
						goto l116
					}
					position++
					goto l99
				l116:
					position, tokenIndex = position99, tokenIndex99
					if buffer[position] != rune(';') {
						goto l117
					}
					position++
					goto l99
				l117:
					position, tokenIndex = position99, tokenIndex99
					if c := buffer[position]; c < rune('<') || c > rune('>') {
						goto l118
					}
					position++
					goto l99
				l118:
					position, tokenIndex = position99, tokenIndex99
					if buffer[position] != rune('?') {
						goto l119
					}
					position++
					goto l99
				l119:
					position, tokenIndex = position99, tokenIndex99
					if buffer[position] != rune('\\') {
						goto l120
					}
					position++
					goto l99
				l120:
					position, tokenIndex = position99, tokenIndex99
					if buffer[position] != rune('^') {
						goto l121
					}
					position++
					goto l99
				l121:
					position, tokenIndex = position99, tokenIndex99
					if buffer[position] != rune('_') {
						goto l122
					}
					position++
					goto l99
				l122:
					position, tokenIndex = position99, tokenIndex99
					if buffer[position] != rune('`') {
						goto l123
					}
					position++
					goto l99
				l123:
					position, tokenIndex = position99, tokenIndex99
					if buffer[position] != rune('{') {
						goto l124
					}
					position++
					goto l99
				l124:
					position, tokenIndex = position99, tokenIndex99
					if buffer[position] != rune('|') {
						goto l125
					}
					position++
					goto l99
				l125:
					position, tokenIndex = position99, tokenIndex99
					if buffer[position] != rune('}') {
						goto l126
					}
					position++
					goto l99
				l126:
					position, tokenIndex = position99, tokenIndex99
					if buffer[position] != rune('~') {
						goto l97
					}
					position++
				}
			l99:
				add(ruletextdata, position98)
			}
			return true
		l97:
			position, tokenIndex = position97, tokenIndex97
			return false
		},
		/* 21 Action0 <- <{ p.pushArray() }> */
		nil,
		/* 22 Action1 <- <{ p.popArray() }> */
		nil,
		nil,
		/* 24 Action2 <- <{ p.addElement(buffer[begin:end]) }> */
		nil,
		/* 25 Action3 <- <{ p.addElement(buffer[begin:end]) }> */
		nil,
		/* 26 Action4 <- <{ p.addElement(buffer[begin:end]) }> */
		nil,
	}
	p.rules = _rules
}
