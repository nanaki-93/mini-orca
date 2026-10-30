package syntax

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	gotreesitter "github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
)

const (
	parseTimeoutMicros = 3_000_000
	parseMemoryBudget  = 256 << 20
)

// grammar is a loaded language with its compiled outline query.
type grammar struct {
	name     string
	language *gotreesitter.Language
	outliner *gotreesitter.Outliner
	// functions reports that the outline query can emit functions.
	functions bool
}

var (
	grammarsMu sync.Mutex
	loaded     = map[string]*grammar{}
)

func analyzeTreeSitter(path string, source []byte) File {
	entry := grammars.DetectLanguage(filepath.Base(path))
	if entry == nil {
		return File{Reason: "no grammar for this file type"}
	}
	g, err := loadGrammar(entry)
	if err != nil {
		return File{Grammar: entry.Name, Reason: err.Error()}
	}
	parser := gotreesitter.NewParser(g.language)
	parser.SetTimeoutMicros(parseTimeoutMicros)
	parser.SetMemoryBudgetBytes(parseMemoryBudget)
	tree, err := parser.ParseStrict(source)
	if tree != nil {
		defer tree.Release()
	}
	if err != nil {
		if errors.Is(err, gotreesitter.ErrParseStoppedEarly) {
			return File{Grammar: g.name, Reason: "parsing stopped early (" + string(tree.ParseStopReason()) + ")"}
		}
		return File{Grammar: g.name, Reason: "parse failed: " + err.Error()}
	}
	result := File{Grammar: g.name, SyntaxError: firstSyntaxError(tree.RootNode(), g.language, source)}
	if !g.functions {
		result.Reason = "the " + g.name + " grammar has no function outline"
		return result
	}
	symbols, report := g.outliner.OutlineTree(tree)
	if report.DeclineReason != "" {
		result.Reason = "outline declined: " + string(report.DeclineReason)
		return result
	}
	result.Measured = true
	measure := functionMeasurer{root: tree.RootNode(), language: g.language, source: source}
	result.Functions = measure.collect(symbols, "")
	return result
}

func loadGrammar(entry *grammars.LangEntry) (*grammar, error) {
	grammarsMu.Lock()
	defer grammarsMu.Unlock()
	if g := loaded[entry.Name]; g != nil {
		return g, nil
	}
	language := entry.Language()
	if language == nil {
		return nil, fmt.Errorf("the %s grammar could not be loaded", entry.Name)
	}
	query := grammars.ResolveTagsQuery(*entry)
	outliner, err := gotreesitter.NewOutliner(language, query+extraDefinitions[entry.Name])
	if err != nil && extraDefinitions[entry.Name] != "" {
		outliner, err = gotreesitter.NewOutliner(language, query)
	}
	if err != nil {
		return nil, fmt.Errorf("the %s outline query does not compile: %w", entry.Name, err)
	}
	g := &grammar{name: entry.Name, language: language, outliner: outliner}
	for _, kind := range outliner.DefinitionKinds() {
		g.functions = g.functions || functionKinds[kind]
	}
	loaded[entry.Name] = g
	return g, nil
}

var functionKinds = map[string]bool{"function": true, "method": true, "constructor": true}

// extraDefinitions adds function shapes the shipped tags queries leave out:
// functions bound to a name in JavaScript and TypeScript (const f = () => {})
// and C++ member functions defined outside their class.
var extraDefinitions = map[string]string{
	"javascript": jsBoundFunctions + "\n(field_definition property: (property_identifier) @name value: [(arrow_function) (function_expression)]) @definition.method\n",
	"typescript": jsBoundFunctions + tsFieldFunctions,
	"tsx":        jsBoundFunctions + tsFieldFunctions,
	"cpp":        "\n(function_definition declarator: (function_declarator declarator: (qualified_identifier) @name)) @definition.function\n",
}

const (
	jsBoundFunctions = `
(variable_declarator name: (identifier) @name value: [(arrow_function) (function_expression) (generator_function)]) @definition.function
(generator_function_declaration name: (identifier) @name) @definition.function
`
	tsFieldFunctions = "(public_field_definition name: (property_identifier) @name value: [(arrow_function) (function_expression)]) @definition.method\n"
)

func firstSyntaxError(root *gotreesitter.Node, language *gotreesitter.Language, source []byte) string {
	if root == nil || !root.HasErrorOrMissing() {
		return ""
	}
	node := root
	for {
		if node.IsMissing() {
			return fmt.Sprintf("line %d: missing %s", node.StartPoint().Row+1, node.Type(language))
		}
		if node.IsError() {
			return fmt.Sprintf("line %d: unexpected %q", node.StartPoint().Row+1, errorExcerpt(node.Text(source)))
		}
		next := firstChildWithError(node)
		if next == nil {
			return fmt.Sprintf("line %d: syntax error", node.StartPoint().Row+1)
		}
		node = next
	}
}

func firstChildWithError(node *gotreesitter.Node) *gotreesitter.Node {
	for i := 0; i < node.ChildCount(); i++ {
		if child := node.Child(i); child != nil && (child.IsError() || child.IsMissing() || child.HasErrorOrMissing()) {
			return child
		}
	}
	return nil
}

func errorExcerpt(text string) string {
	text = strings.Join(strings.Fields(text), " ")
	if len(text) > 40 {
		return strings.TrimSpace(text[:40]) + "…"
	}
	return text
}
