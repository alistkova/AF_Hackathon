export type User = {
  id: string;
  email: string;
  role: "admin" | "analyst";
};

export type Company = {
  id: string;
  name: string;
  inn: string;
  industry: string;
  region: string;
  description: string;
  created_at: string;
};

export type FeedItem = {
  event_id: string;
  company_id: string;
  title: string;
  summary: string;
  source: string;
  url: string;
  published_at: string;
  relevance: number;
  reasons: string[];
  tags: string[];
  target_sectors: string[];
};

export type FeedFacet = {
  value: string;
  count: number;
};

export type FeedFacets = {
  sources: FeedFacet[];
  tags: FeedFacet[];
  sectors: FeedFacet[];
};

export type FeedFilters = {
  source?: string;
  tag?: string;
  sector?: string;
  query?: string;
  from?: string;
  to?: string;
};

export type Source = {
  source_id: string;
  title: string;
  url: string;
};

export type Insight = {
  summary: string;
  why_it_matters: string;
  risks: string[];
  opportunities: string[];
  bank_position: string;
  ai_conclusion: string;
  sources: Source[];
  actions: string[];
  confidence: number;
};

export type InsightJob = {
  job_id: string;
  company_id: string;
  status: "queued" | "running" | "done" | "failed";
  events_url: string;
  result_url: string;
  insight?: Insight;
  error?: string;
};

export type StepEvent = {
  type: "step";
  job_id: string;
  seq: number;
  name: string;
  status: "running" | "done";
  message: string;
  ts: string;
};

export type ResultEvent = {
  type: "result";
  job_id: string;
  seq: number;
  insight: Insight;
  ts: string;
};

export type ErrorEvent = {
  type: "error";
  job_id: string;
  seq: number;
  code: string;
  message: string;
  retryable?: boolean;
  ts: string;
};
