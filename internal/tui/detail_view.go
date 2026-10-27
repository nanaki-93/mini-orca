package tui

import (
	"context"
	"net/http"
	"net/url"
)

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
