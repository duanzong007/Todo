export interface UserView {
  display_name: string;
  username: string;
  is_admin: boolean;
}

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

export interface AccountFilterOption {
  value: string;
  label: string;
  selected: boolean;
}

export interface AccountCheckOption {
  value: string;
  label: string;
  checked: boolean;
}

export interface AccountTaskFilterView {
  query: string;
  summary: string;
  limit_value: string;
  page_value: string;
  date_from: string;
  date_to: string;
  status_options: AccountFilterOption[];
  scope_options: AccountFilterOption[];
  date_field_options: AccountFilterOption[];
  sort_options: AccountFilterOption[];
  limit_options: AccountFilterOption[];
  type_options: AccountCheckOption[];
  importance_options: AccountCheckOption[];
}

export interface AccountPaginationView {
  page: number;
  total_pages: number;
  total_items: number;
  has_pages: boolean;
  has_prev: boolean;
  has_next: boolean;
  prev_page: number;
  next_page: number;
  page_options: AccountFilterOption[];
}

export interface ManagedTaskCard {
  id: string;
  title: string;
  kind_label: string;
  kind_class: string;
  importance: number;
  status_label: string;
  status_class: string;
  date_line: string;
  shared_line: string;
  note: string;
  is_owner: boolean;
  shared_with_me: boolean;
  schedule_mode: "none" | "date" | "datetime" | string;
  schedule_value: string;
  deadline_date: string;
  deadline_time: string;
}

export interface ShareableUserCard {
  id: string;
  display_name: string;
  username: string;
}

export interface AccountPageData {
  current_user: UserView;
  message: string;
  error: string;
  return_query: string;
  today_date_iso: string;
  filter: AccountTaskFilterView;
  pagination: AccountPaginationView;
  tasks: ManagedTaskCard[];
  share_users: ShareableUserCard[];
}

export interface AccountActionResponse {
  message?: string;
  error?: string;
}

export type ConnectionState = "idle" | "loading" | "ready" | "unauthorized" | "error";
