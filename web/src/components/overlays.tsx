// Dialogs, confirmations and toasts. Confirmation is promise-based so actions
// read top to bottom: `if (!(await confirm({...}))) return;`.

import { createContext, useCallback, useContext, useMemo, useRef, useState, type ReactNode } from 'react';
import * as Dialog from '@radix-ui/react-dialog';
import * as AlertDialog from '@radix-ui/react-alert-dialog';
import { AlertTriangle, CheckCircle2, Info, X, XCircle } from 'lucide-react';
import { Button, cx } from './ui';

export function Modal({ open, onOpenChange, title, description, children, footer, size = 'md' }: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  title: ReactNode;
  description?: ReactNode;
  children: ReactNode;
  footer?: ReactNode;
  size?: 'sm' | 'md' | 'lg';
}) {
  return (
    <Dialog.Root open={open} onOpenChange={onOpenChange}>
      <Dialog.Portal>
        <Dialog.Overlay className="overlay" />
        <Dialog.Content className={cx('modal', `modal-${size}`)} aria-describedby={description ? undefined : undefined}>
          <div className="modal-head">
            <div className="grow">
              <Dialog.Title className="modal-title">{title}</Dialog.Title>
              {description ? <Dialog.Description className="modal-desc">{description}</Dialog.Description> : <Dialog.Description className="sr-only">{title}</Dialog.Description>}
            </div>
            <Dialog.Close asChild>
              <button type="button" className="icon-btn" aria-label="Close">
                <X size={16} aria-hidden />
              </button>
            </Dialog.Close>
          </div>
          <div className="modal-body">{children}</div>
          {footer ? <div className="modal-foot">{footer}</div> : null}
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  );
}

export interface ConfirmOptions {
  title: string;
  body?: ReactNode;
  confirmLabel?: string;
  cancelLabel?: string;
  danger?: boolean;
}

type ToastTone = 'info' | 'success' | 'warning' | 'danger';
interface Toast {
  id: number;
  tone: ToastTone;
  message: string;
  detail?: string;
}

interface Feedback {
  confirm: (options: ConfirmOptions) => Promise<boolean>;
  toast: (message: string, tone?: ToastTone, detail?: string) => void;
}

const FeedbackContext = createContext<Feedback | null>(null);

export function useFeedback(): Feedback {
  const value = useContext(FeedbackContext);
  if (!value) throw new Error('useFeedback outside FeedbackProvider');
  return value;
}

const TOAST_ICON = { info: Info, success: CheckCircle2, warning: AlertTriangle, danger: XCircle };

export function FeedbackProvider({ children }: { children: ReactNode }) {
  const [pending, setPending] = useState<(ConfirmOptions & { resolve: (ok: boolean) => void }) | null>(null);
  const [toasts, setToasts] = useState<Toast[]>([]);
  const nextId = useRef(1);

  const confirm = useCallback((options: ConfirmOptions) => new Promise<boolean>(resolve => setPending({ ...options, resolve })), []);
  const dismiss = useCallback((id: number) => setToasts(list => list.filter(t => t.id !== id)), []);
  const toast = useCallback(
    (message: string, tone: ToastTone = 'info', detail?: string) => {
      const id = nextId.current++;
      setToasts(list => [...list.slice(-3), { id, tone, message, detail }]);
      window.setTimeout(() => dismiss(id), tone === 'danger' ? 9000 : 4500);
    },
    [dismiss],
  );
  const value = useMemo(() => ({ confirm, toast }), [confirm, toast]);

  const close = (ok: boolean) => {
    pending?.resolve(ok);
    setPending(null);
  };

  return (
    <FeedbackContext.Provider value={value}>
      {children}
      <AlertDialog.Root open={Boolean(pending)} onOpenChange={open => !open && close(false)}>
        <AlertDialog.Portal>
          <AlertDialog.Overlay className="overlay" />
          <AlertDialog.Content className="modal modal-sm">
            <div className="modal-head">
              <AlertDialog.Title className="modal-title">{pending?.title}</AlertDialog.Title>
            </div>
            <AlertDialog.Description asChild>
              <div className="modal-body confirm-body">{pending?.body}</div>
            </AlertDialog.Description>
            <div className="modal-foot">
              <AlertDialog.Cancel asChild>
                <Button variant="ghost">{pending?.cancelLabel ?? 'Cancel'}</Button>
              </AlertDialog.Cancel>
              <AlertDialog.Action asChild>
                <Button variant={pending?.danger ? 'danger' : 'primary'} onClick={() => close(true)} autoFocus>
                  {pending?.confirmLabel ?? 'Confirm'}
                </Button>
              </AlertDialog.Action>
            </div>
          </AlertDialog.Content>
        </AlertDialog.Portal>
      </AlertDialog.Root>
      <div className="toasts" role="region" aria-label="Notifications" aria-live="polite">
        {toasts.map(t => {
          const Icon = TOAST_ICON[t.tone];
          return (
            <div key={t.id} className={cx('toast', `toast-${t.tone}`)} role={t.tone === 'danger' ? 'alert' : 'status'}>
              <Icon size={16} className="toast-icon" aria-hidden />
              <div className="grow">
                <div className="toast-message">{t.message}</div>
                {t.detail ? <div className="toast-detail">{t.detail}</div> : null}
              </div>
              <button type="button" className="icon-btn icon-btn-sm" aria-label="Dismiss" onClick={() => dismiss(t.id)}>
                <X size={14} aria-hidden />
              </button>
            </div>
          );
        })}
      </div>
    </FeedbackContext.Provider>
  );
}
