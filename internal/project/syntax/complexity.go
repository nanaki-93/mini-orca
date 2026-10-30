package syntax

import (
	"strings"

	gotreesitter "github.com/odvcencio/gotreesitter"
)

// functionMeasurer turns outline symbols into measured functions.
type functionMeasurer struct {
	root     *gotreesitter.Node
	language *gotreesitter.Language
	source   []byte
}

func (m functionMeasurer) collect(symbols []gotreesitter.OutlineSymbol, prefix string) []Function {
	var functions []Function
	for _, symbol := range symbols {
		name := symbol.Name
		if prefix != "" {
			name = prefix + "." + name
		}
		if functionKinds[symbol.Kind] {
			if node := m.definitionNode(symbol); node != nil {
				functions = append(functions, Function{
					Name: name, Kind: symbol.Kind,
					StartLine:  int(symbol.Range.StartPoint.Row) + 1,
					EndLine:    int(symbol.Range.EndPoint.Row) + 1,
					Complexity: 1 + m.decisions(node, symbol.Children),
					Hash:       textHash(string(m.source[symbol.Range.StartByte:symbol.Range.EndByte])),
				})
			}
		}
		functions = append(functions, m.collect(symbol.Children, name)...)
	}
	return functions
}

// definitionNode finds the syntax node an outline symbol was read from.
func (m functionMeasurer) definitionNode(symbol gotreesitter.OutlineSymbol) *gotreesitter.Node {
	node := m.root.NamedDescendantForByteRange(symbol.Range.StartByte, symbol.Range.EndByte)
	for node != nil && node.Type(m.language) != symbol.NodeType {
		parent := node.Parent()
		if parent == nil || parent.StartByte() != symbol.Range.StartByte || parent.EndByte() != symbol.Range.EndByte {
			return nil
		}
		node = parent
	}
	return node
}

// decisions counts the decision points below node. Definitions nested in it
// (an inner function, a local class) are measured on their own and skipped.
func (m functionMeasurer) decisions(node *gotreesitter.Node, nested []gotreesitter.OutlineSymbol) int {
	count := 0
	for i := 0; i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child == nil || isNestedDefinition(child, nested) {
			continue
		}
		if m.isDecision(child) {
			count++
		}
		count += m.decisions(child, nested)
	}
	return count
}

func isNestedDefinition(node *gotreesitter.Node, nested []gotreesitter.OutlineSymbol) bool {
	for _, symbol := range nested {
		if node.StartByte() == symbol.Range.StartByte && node.EndByte() == symbol.Range.EndByte {
			return true
		}
	}
	return false
}

func (m functionMeasurer) isDecision(node *gotreesitter.Node) bool {
	kind := node.Type(m.language)
	if !node.IsNamed() {
		return booleanOperators[kind] && node.Parent() != nil && isBooleanExpression(node.Parent().Type(m.language))
	}
	if branchNodes[kind] {
		return true
	}
	return caseNodes[kind] && !m.isDefaultCase(node)
}

// isDefaultCase reports the fallback arm of a switch or match (default:,
// else ->, _ =>), which gocyclo does not count either.
func (m functionMeasurer) isDefaultCase(node *gotreesitter.Node) bool {
	if first := node.Child(0); first != nil {
		switch first.Type(m.language) {
		case "default", "else", "default_keyword", "default_switch_label":
			return true
		}
	}
	if first := node.NamedChild(0); first != nil {
		return strings.TrimSpace(first.Text(m.source)) == "_"
	}
	return false
}

func isBooleanExpression(kind string) bool {
	for _, marker := range []string{"binary", "boolean", "infix", "logical"} {
		if strings.Contains(kind, marker) {
			return true
		}
	}
	return false
}

// branchNodes are the conditionals, loops, handlers and conditional
// expressions of the embedded grammars; each one adds a path.
var branchNodes = setOf(
	"if_statement", "if_expression", "elif_clause", "else_if_clause", "if_modifier", "unless", "unless_modifier",
	"guard_statement", "if_clause", "if_element",
	"for_statement", "for_in_statement", "enhanced_for_statement", "for_expression", "for_range_loop",
	"foreach_statement", "c_style_for_statement", "for_in_clause", "for_loop", "for_in_loop", "for_element",
	"while_statement", "while_expression", "while_modifier", "while_loop", "until", "until_modifier",
	"do_statement", "do_while_statement", "do_while_expression", "do_while_loop", "loop_expression",
	"repeat_statement", "repeat_while_statement",
	"catch_clause", "except_clause", "rescue", "rescue_modifier", "catch_block",
	"conditional_expression", "ternary_expression", "ternary_op", "conditional",
)

// caseNodes are switch and match arms; the default arm is not counted.
var caseNodes = setOf(
	"switch_case", "case_clause", "switch_label", "when_entry", "match_arm", "switch_section",
	"switch_expression_arm", "case_statement", "when", "case_item", "switch_entry", "match_case",
	"match_conditional_expression", "switch_statement_case", "switch_expression_case",
)

// booleanOperators short-circuit, so each one adds a path.
var booleanOperators = setOf("&&", "||", "and", "or", "??", "andalso", "orelse")

func setOf(items ...string) map[string]bool {
	set := make(map[string]bool, len(items))
	for _, item := range items {
		set[item] = true
	}
	return set
}
