package tui

import (
	"context"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/nanaki-93/mini-orca/v2/internal/tui/client"
)

type mediaProgressRequest struct {
	Parent           ProjectIdentity
	Scope            client.AgentSettingsScope
	Epoch, SessionID string
}
type mediaStreamUpdate struct {
	stream *mediaStream
	value  *client.LiveSession
	err    error
}
type mediaStream struct {
	request mediaProgressRequest
	updates chan mediaStreamUpdate
	done    chan struct{}
	cancel  context.CancelFunc
}

func startMediaStream(ctx context.Context, cl *Client, request mediaProgressRequest) *mediaStream {
	owned, cancel := context.WithCancel(ctx)
	stream := &mediaStream{request: request, updates: make(chan mediaStreamUpdate, 1), done: make(chan struct{}), cancel: cancel}
	go stream.run(owned, cl)
	return stream
}

func (stream *mediaStream) run(ctx context.Context, cl *Client) {
	defer close(stream.done)
	defer close(stream.updates)
	for {
		bounded, cancel := context.WithTimeout(ctx, 10*time.Second)
		value, err := cl.LiveSessionDetail(bounded, nextTUIReqID("live-progress"), stream.request.Scope, stream.request.SessionID)
		cancel()
		if err == nil && value == nil {
			err = fmt.Errorf("live session state unavailable")
		}
		select {
		case stream.updates <- mediaStreamUpdate{stream: stream, value: value, err: err}:
		case <-ctx.Done():
			return
		}
		if err != nil {
			return
		}
		timer := time.NewTimer(2 * time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

func (stream *mediaStream) stop() { stream.cancel(); <-stream.done }

func waitMediaStream(stream *mediaStream) tea.Cmd {
	return func() tea.Msg {
		update, ok := <-stream.updates
		if !ok {
			return mediaStreamUpdate{stream: stream}
		}
		return update
	}
}

func (s *State) mediaProgressRequest() *mediaProgressRequest {
	if !s.mediaCurrent() || s.Page != PageMedia || s.AgentMedia.Pending != "" || s.AgentMedia.Failures["sessions"] != "" {
		return nil
	}
	item := s.selectedMediaSession()
	if item == nil || !item.Owned || item.Stale {
		return nil
	}
	v := s.AgentMedia
	return &mediaProgressRequest{Parent: v.Parent, Scope: v.Scope, Epoch: v.Epoch, SessionID: item.ID}
}

func (m *BubbleModel) syncMediaStream(cmds *[]tea.Cmd) {
	request := m.state.mediaProgressRequest()
	if !m.nativeStreams || m.client == nil || request == nil {
		m.stopMediaStream()
		return
	}
	if m.mediaStream != nil && m.mediaStream.request == *request {
		return
	}
	m.stopMediaStream()
	m.mediaStream = startMediaStream(m.ctx, m.client, *request)
	*cmds = append(*cmds, waitMediaStream(m.mediaStream))
}

func (m *BubbleModel) stopMediaStream() {
	if m.mediaStream != nil {
		m.mediaStream.stop()
		m.mediaStream = nil
	}
}

func (m *BubbleModel) applyMediaStreamUpdate(update mediaStreamUpdate) tea.Cmd {
	request := m.state.mediaProgressRequest()
	if update.stream == nil || update.stream != m.mediaStream || request == nil || *request != update.stream.request {
		return nil
	}
	v := m.state.AgentMedia
	if update.err != nil {
		v.Failures["sessions"] = externalSafeText(update.err.Error())
		v.Notice = "Live state unavailable. Refresh local history; no reconnection or input resend has occurred."
		return nil
	}
	if update.value == nil {
		return nil
	}
	if update.value.LiveBinding != v.Session.LiveBinding || update.value.ControlRevision < v.Session.ControlRevision {
		v.Failures["sessions"] = "Live identity changed. Refresh local history."
		return nil
	}
	if update.value.Version >= v.Session.Version {
		v.Session = update.value
		m.state.mergeMediaSession(*update.value)
	}
	return waitMediaStream(m.mediaStream)
}

func (s *State) reconcileMediaScope() {
	if s.AgentMedia == nil || s.mediaCurrent() {
		return
	}
	s.cancelMediaRequest()
	s.AgentMedia.Private, s.AgentMedia.Import = nil, nil
}
