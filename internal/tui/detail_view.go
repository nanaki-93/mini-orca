package tui

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/nanaki-93/mini-orca/v2/internal/tui/theme"
)

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
