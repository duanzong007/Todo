export interface QuoteView {
  text: string;
  author: string;
  source: string;
  has_meta: boolean;
  meta_line: string;
}

export interface TaskCard {
  id: string;
  title: string;
  kind_label: string;
  kind_class: string;
  importance: number;
  status_line: string;
  compact_status: string;
  note: string;
  can_complete: boolean;
  can_postpone: boolean;
  postpone_mode: string;
  postpone_value: string;
  postpone_min_value: string;
  return_date: string;
}

export interface CompletedTaskCard {
  id: string;
  title: string;
  kind_label: string;
  kind_class: string;
  importance: number;
  finished_line: string;
  status_line: string;
  note: string;
  can_postpone: boolean;
  postpone_mode: string;
  postpone_value: string;
  postpone_min_value: string;
  return_date: string;
}

export interface DashboardSnapshot {
  focus_tasks: TaskCard[];
  completed_tasks: CompletedTaskCard[];
  empty_quote?: QuoteView;
}

export type ConnectionState = "idle" | "loading" | "ready" | "unauthorized" | "error";
