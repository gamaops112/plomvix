export type ToastKind = 'success' | 'error' | 'warning' | 'info';

export type ToastInput = {
  kind?: ToastKind;
  title: string;
  message?: string;
  durationMs?: number;
};

export type ToastItem = Required<Pick<ToastInput, 'kind' | 'title' | 'durationMs'>> & {
  id: string;
  message?: string;
};

export type AppEvent =
  | { type: 'toast:add'; payload: ToastInput }
  | { type: 'toast:remove'; payload: { id: string } }
  | { type: 'app:ready' };
