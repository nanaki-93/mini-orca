package tui

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/nanaki-93/mini-orca/v2/internal/tui/theme"
)

func (s *State) hasScrollableDetails() bool {
	return len(s.detailSections()) > 0 || s.EffectivePage() == PageModels || s.Page == PageWorktreeDetail || s.Page == PageAgents && s.AgentDetailOpen
}

func (s *State) handleDetailScrollKey(ev KeyEvent) bool {
	if !s.hasScrollableDetails() && !(s.Page == PageEditor && s.isViewingDiff()) {
		return false
	}
	step := 8
	if s.Page == PageEditor {
		step = s.diffMetadataRows()
	}
	switch ev.Key {
	case "PageDown":
		s.DetailScroll += step
	case "PageUp":
		s.DetailScroll = max(0, s.DetailScroll-step)
	default:
		return false
	}
	return true
}

func (s *State) scrollDetails(th ansiTheme, body []string, rows int) []string {
	if !s.hasScrollableDetails() {
		return body
	}
	if len(body) <= rows {
		s.DetailScroll = 0
		return body
	}
	visible := max(1, rows-1)
	s.DetailScroll = min(s.DetailScroll, max(0, len(body)-visible))
	start := s.DetailScroll
	out := append([]string(nil), body[start:min(len(body), start+visible)]...)
	return append(out, fmt.Sprintf("  %s[PgUp/PgDn] Scroll details · lines %d–%d of %d%s", th.cyan, start+1, min(len(body), start+visible), len(body), th.reset))
}

func (s *State) issueSourceExcerpt(cur IssueRow, th ansiTheme, contentW int) []string {
	if s.IssueFile == nil || s.IssueFile.Path != cur.Path {
		return nil
	}
	raw := strings.Split(strings.ReplaceAll(s.IssueFile.Content, "\r\n", "\n"), "\n")
	start := max(0, cur.Line-3)
	end := min(len(raw), min(cur.Line+8, max(cur.Line+2, cur.EndLine)))
	var excerpt []string
	for i := start; i < end; i++ {
		excerpt = append(excerpt, fmt.Sprintf("%d: %s", i+1, theme.SanitizeCodeText(raw[i])))
	}
	return renderDiagnosticBulletRows(th, th.cyan, "Source snapshot:", th.cyan, th.fg, excerpt, contentW)
}

func (s *State) queueIssueSource(issue IssueRow) {
	s.DetailScroll = 0
	if issue.Path == "" {
		return
	}
	s.IssueFile = nil
	s.enqueueAction(ActionRequest{Kind: "issue-source", Target: issue.Path, Source: ProjectIdentity{ProjectID: s.projectID(), ProjectRevision: s.projectRevision()}})
}

func (s *State) fetchIssueSource(ctx context.Context, cl *Client, reqID string, action ActionRequest) error {
	if !s.matchesPlanSource(action.Source) {
		return nil
	}
	var info FileInfo
	err := cl.DoJSON(ctx, reqID, http.MethodGet, "/api/projects/current/files/info?path="+url.QueryEscape(action.Target), nil, &info)
	if err != nil {
		return err
	}
	issue := s.SelectedIssue()
	if s.Page == PageIssueDetail && issue != nil && issue.Path == info.Path && s.matchesPlanSource(action.Source) {
		s.IssueFile = &info
	}
	return nil
}
