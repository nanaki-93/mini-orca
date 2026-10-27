// Live updates. The daemon streams `update` events whose kind names what
// changed; each kind invalidates the queries that depend on it. The stream is
// read with fetch so it carries the Web UI header like every other request.

import { useEffect, useState } from 'react';
import { useQueryClient, type QueryClient } from '@tanstack/react-query';

export type Connection = 'connecting' | 'live' | 'offline';

const INVALIDATES: Record<string, string[][]> = {
  workflow: [['session'], ['changes'], ['git']],
  project_index: [['project'], ['index'], ['changes'], ['session'], ['overview'], ['trust'], ['git'], ['file']],
  findings: [['findings'], ['overview']],
  analysis: [['overview'], ['findings']],
  features: [['overview']],
  scan: [['scan']],
  agent_settings: [['settings']],
  missions: [['changes'], ['git']],
  agent_run: [['session']],
};

export function applyEvent(client: QueryClient, kind: string): void {
  for (const key of INVALIDATES[kind] ?? []) void client.invalidateQueries({ queryKey: key });
}

/** Parses complete SSE frames from `buffer`; returns the unparsed tail. */
export function parseFrames(buffer: string, onEvent: (event: string, data: string) => void): string {
  const frames = buffer.split('\n\n');
  const tail = frames.pop() ?? '';
  for (const frame of frames) {
    let event = 'message';
    const data: string[] = [];
    for (const line of frame.split('\n')) {
      if (line.startsWith('event:')) event = line.slice(6).trim();
      else if (line.startsWith('data:')) data.push(line.slice(5).trim());
    }
    if (data.length) onEvent(event, data.join('\n'));
  }
  return tail;
}

export function useDaemonEvents(): Connection {
  const client = useQueryClient();
  const [connection, setConnection] = useState<Connection>('connecting');
  useEffect(() => {
    let stopped = false;
    let controller: AbortController | null = null;
    let retry = 1000;
    async function connect() {
      while (!stopped) {
        controller = new AbortController();
        try {
          const response = await fetch('/api/projects/current/events', {
            headers: { Accept: 'text/event-stream', 'X-Mini-Orca-Web-UI': '1' },
            signal: controller.signal,
          });
          if (!response.ok || !response.body) throw new Error('events unavailable');
          const reader = response.body.getReader();
          const decoder = new TextDecoder();
          let buffer = '';
          setConnection('live');
          retry = 1000;
          for (;;) {
            const { value, done } = await reader.read();
            if (done) break;
            buffer = parseFrames(buffer + decoder.decode(value, { stream: true }), (event, data) => {
              if (event !== 'update') return;
              try {
                applyEvent(client, (JSON.parse(data) as { kind: string }).kind);
              } catch {
                // A malformed frame carries no actionable kind.
              }
            });
          }
        } catch {
          if (stopped) return;
        }
        if (stopped) return;
        setConnection('offline');
        await new Promise(resolve => setTimeout(resolve, retry));
        retry = Math.min(15_000, retry * 2);
        setConnection('connecting');
      }
    }
    void connect();
    return () => {
      stopped = true;
      controller?.abort();
    };
  }, [client]);
  return connection;
}
