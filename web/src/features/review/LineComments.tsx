// Line comments in Files changed: the reviewer's requests on specific lines.
// Open comments go to the agents as one repair brief with the next revision
// or agent run; they change no source and no review state.

import { useMemo, useState } from 'react';
import { MessageSquarePlus, Send, Trash2 } from 'lucide-react';
import type { ChangeComment, LoadedSession } from '@/api/types';
import { useActions, useBusy } from '@/app/actions';
import { anchorKey, type LineAnchor } from '@/lib/diff';
import { openComments } from '@/lib/readiness';
import { plural, timeAgo } from '@/lib/util';
import { Button, IconButton, Textarea } from '@/components/ui';
import type { DiffComments } from './DiffView';

const MAX_COMMENT = 1024;

/** Comments for one file of the current proposal, wired for DiffView. */
export function useFileComments(session: LoadedSession, path: string, canComment: boolean): DiffComments {
  const [draft, setDraft] = useState<LineAnchor | null>(null);
  const comments = useMemo(() => (session.comments ?? []).filter(comment => comment.path === path && comment.hash === session.hash), [session.comments, session.hash, path]);
  const anchored = useMemo(() => {
    const keys = new Set(comments.map(comment => anchorKey(comment)));
    if (draft) keys.add(anchorKey(draft));
    return keys;
  }, [comments, draft]);
  return {
    anchored,
    onAdd: canComment ? setDraft : undefined,
    render: anchor => (
      <div className="line-comments">
        {comments
          .filter(comment => anchorKey(comment) === anchorKey(anchor))
          .map(comment => (
            <LineComment key={comment.id} session={session} comment={comment} />
          ))}
        {draft && anchorKey(draft) === anchorKey(anchor) ? <CommentForm session={session} path={path} anchor={anchor} onDone={() => setDraft(null)} /> : null}
      </div>
    ),
  };
}

function LineComment({ session, comment }: { session: LoadedSession; comment: ChangeComment }) {
  const actions = useActions();
  const busy = useBusy();
  return (
    <div className="line-comment">
      <div className="row">
        <span className="xs strong">You</span>
        <span className="xs faint">{timeAgo(comment.created_at)}</span>
        {comment.sent_revision ? <span className="xs muted">· sent to the agents</span> : null}
        <span className="spacer" />
        {!comment.sent_revision ? <IconButton icon={Trash2} label="Delete comment" size="sm" onClick={() => void actions.deleteComment(session, comment.id)} disabled={Boolean(busy)} /> : null}
      </div>
      <p className="prose small">{comment.text}</p>
    </div>
  );
}

function CommentForm({ session, path, anchor, onDone }: { session: LoadedSession; path: string; anchor: LineAnchor; onDone: () => void }) {
  const actions = useActions();
  const busy = useBusy();
  const [text, setText] = useState('');
  const submit = async () => {
    if (text.trim() && (await actions.addComment(session, { path, ...anchor }, text))) onDone();
  };
  return (
    <div className="comment-form">
      <Textarea
        rows={3}
        value={text}
        maxLength={MAX_COMMENT}
        onChange={e => setText(e.target.value)}
        onKeyDown={e => {
          if ((e.metaKey || e.ctrlKey) && e.key === 'Enter') {
            e.preventDefault();
            void submit();
          } else if (e.key === 'Escape') {
            e.preventDefault();
            onDone();
          }
        }}
        placeholder="What should change on this line?"
        aria-label={`Comment on line ${anchor.line}`}
        autoFocus
      />
      <div className="row">
        <span className="xs faint">Comments go to the agents with the next revision or run.</span>
        <span className="spacer" />
        <Button size="sm" variant="ghost" onClick={onDone}>
          Cancel
        </Button>
        <Button size="sm" variant="primary" icon={MessageSquarePlus} onClick={() => void submit()} disabled={!text.trim() || Boolean(busy)} kbd="mod+Enter">
          Comment
        </Button>
      </div>
    </div>
  );
}

/** The open comments of a task and the action that sends them as a revision. */
export function SendComments({ session, disabled }: { session: LoadedSession; disabled: boolean }) {
  const actions = useActions();
  const busy = useBusy();
  const open = openComments(session).length;
  if (!open) return null;
  return (
    <Button size="sm" icon={Send} onClick={() => void actions.refine(session, '', true)} disabled={disabled || Boolean(busy)} loading={busy === 'Generating…'}>
      Revise with {plural(open, 'comment')}
    </Button>
  );
}
