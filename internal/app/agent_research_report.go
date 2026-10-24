package app

import (
	"bytes"
	"cmp"
	"fmt"
	"html"
	"html/template"
	"io"
	"net/url"
	"slices"
	"strings"
	"unicode/utf8"

	htmlparser "golang.org/x/net/html"
	"golang.org/x/net/html/atom"

	"github.com/nanaki-93/mini-orca/v2/internal/app/agentruntime"
)

const ResearchReportCSP = "default-src 'none'; style-src 'unsafe-inline'; img-src data:; frame-src 'self'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'; sandbox allow-popups allow-popups-to-escape-sandbox"

const researchReportTemplate = `<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><meta http-equiv="Content-Security-Policy" content="default-src 'none'; style-src 'unsafe-inline'; img-src data:; frame-src 'self'; base-uri 'none'; form-action 'none'"><title>Mini-Orca research</title>
<style>:root{color-scheme:light dark}body{font:16px/1.65 system-ui,sans-serif;margin:3rem auto;padding:0 1.5rem;max-width:72ch}h1{font-size:1.8rem;margin-bottom:.4rem}h2{font-size:1.1rem;margin-top:2rem}.meta{opacity:.7;font-size:.9rem}.answer{white-space:pre-wrap;overflow-wrap:anywhere}a{color:light-dark(#1650a5,#8ab4f8)}sup{font-size:.75rem;margin-left:.2rem}iframe{display:block;width:100%;height:240px;border:0;margin-top:.5rem}li{margin:.6rem 0;overflow-wrap:anywhere}.notice{border-left:3px solid currentColor;padding-left:1rem}</style></head>
<body><p class="meta">MINI-ORCA · EXTERNAL RESEARCH</p><h1>{{.Question}}</h1><p class="meta">{{.Report.Model}} · {{.Report.State}} · {{.Report.CreatedAt.Format "2006-01-02 15:04 UTC"}}</p>
<p class="notice">This answer is advisory. External citations do not establish verified repository findings.{{if .Report.Stale}} This report belongs to an earlier project snapshot.{{end}}</p>
<div class="answer">{{.AnswerHTML}}</div>
{{if .Grounding.SuggestionsHTML}}<h2>Google Search suggestions</h2><iframe title="Google Search suggestions" sandbox="allow-popups allow-popups-to-escape-sandbox" referrerpolicy="no-referrer" srcdoc="{{.Grounding.SuggestionsHTML}}"></iframe>{{end}}
{{if .Grounding.Sources}}<h2>Sources</h2><ol>{{range .Grounding.Sources}}<li><a href="{{.URL}}" target="_blank" rel="noopener noreferrer">{{if .Title}}{{.Title}}{{else}}{{.URL}}{{end}}</a></li>{{end}}</ol>{{end}}
{{if .Grounding.Queries}}<h2>Reported search queries</h2><ul>{{range .Grounding.Queries}}<li>{{.}}</li>{{end}}</ul>{{end}}
<p class="meta">Provider-internal search usage is unknown. Query metadata is not a billing receipt. Model token usage: {{if .UsageKnown}}{{.Usage.PromptTokens}} input / {{.Usage.CompletionTokens}} output{{else}}unknown{{end}}.</p>
</body></html>`

func renderResearchReport(document researchDocument) ([]byte, error) {
	if document.Grounding == nil || len(document.Answer) > 64<<10 || !utf8.ValidString(document.Answer) {
		return nil, fmt.Errorf("invalid research report")
	}
	if err := validateSearchSuggestions(document.Grounding.SuggestionsHTML); err != nil {
		return nil, err
	}
	answer, err := researchAnswerHTML(document.Answer, document.Grounding)
	if err != nil {
		return nil, err
	}
	view := struct {
		researchDocument
		AnswerHTML template.HTML
	}{researchDocument: document, AnswerHTML: template.HTML(answer)}
	page, err := template.New("research").Parse(researchReportTemplate)
	if err != nil {
		return nil, err
	}
	var output bytes.Buffer
	if err := page.Execute(&output, view); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

func researchAnswerHTML(text string, grounding *agentruntime.GroundingInfo) (string, error) {
	if len(grounding.Sources) > 32 || len(grounding.Supports) > 64 {
		return "", fmt.Errorf("research attribution exceeds limits")
	}
	for _, source := range grounding.Sources {
		if !researchLink(source.URL) {
			return "", fmt.Errorf("unsupported research source link")
		}
	}
	supports := slices.Clone(grounding.Supports)
	slices.SortFunc(supports, func(a, b agentruntime.GroundSupport) int { return cmp.Compare(a.End, b.End) })
	var result strings.Builder
	offset := 0
	for _, support := range supports {
		if support.Start < 0 || support.End <= support.Start || support.End > len(text) || !utf8.ValidString(text[support.Start:support.End]) {
			return "", fmt.Errorf("invalid research citation range")
		}
		result.WriteString(html.EscapeString(text[offset:support.End]))
		for _, index := range support.Sources {
			if index < 0 || index >= len(grounding.Sources) {
				return "", fmt.Errorf("invalid research citation index")
			}
			fmt.Fprintf(&result, `<sup><a href="%s" target="_blank" rel="noopener noreferrer">[%d]</a></sup>`, html.EscapeString(grounding.Sources[index].URL), index+1)
		}
		offset = support.End
	}
	result.WriteString(html.EscapeString(text[offset:]))
	return result.String(), nil
}

func researchLink(value string) bool {
	u, err := url.Parse(value)
	return err == nil && len(value) <= 4096 && u.Hostname() != "" && u.User == nil && (u.Scheme == "http" || u.Scheme == "https") && !strings.ContainsAny(value, "\x00\r\n")
}

func validateSearchSuggestions(fragment string) error {
	if len(fragment) > 64<<10 || !utf8.ValidString(fragment) {
		return fmt.Errorf("search suggestions exceed limits")
	}
	tokens := htmlparser.NewTokenizer(strings.NewReader(fragment))
	for {
		kind := tokens.Next()
		if kind == htmlparser.ErrorToken {
			if tokens.Err() == io.EOF {
				return nil
			}
			return fmt.Errorf("invalid search suggestion HTML")
		}
		if kind != htmlparser.StartTagToken && kind != htmlparser.SelfClosingTagToken {
			continue
		}
		token := tokens.Token()
		if slices.Contains([]atom.Atom{atom.Script, atom.Iframe, atom.Frame, atom.Frameset, atom.Object, atom.Embed, atom.Meta, atom.Base, atom.Form, atom.Link}, token.DataAtom) {
			return fmt.Errorf("search suggestion HTML requires unsupported active content")
		}
		for _, attribute := range token.Attr {
			if err := validateSuggestionAttribute(attribute); err != nil {
				return err
			}
		}
	}
}

func validateSuggestionAttribute(attribute htmlparser.Attribute) error {
	key := strings.ToLower(attribute.Key)
	if strings.HasPrefix(key, "on") || slices.Contains([]string{"srcset", "action", "formaction", "ping"}, key) {
		return fmt.Errorf("unsupported active search suggestion attribute")
	}
	if key == "src" && !strings.HasPrefix(attribute.Val, "data:image/") {
		return fmt.Errorf("search suggestions cannot load remote resources during preview")
	}
	if key == "href" && !strings.HasPrefix(attribute.Val, "#") && !researchLink(attribute.Val) {
		return fmt.Errorf("unsupported search suggestion link")
	}
	return nil
}
