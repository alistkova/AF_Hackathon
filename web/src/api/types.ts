export type User = {
  id: string;
  email: string;
  role: "admin" | "analyst";
  primary_company_id: string;
};

export type ProfileUpdate = {
  email?: string;
  primary_company_id?: string;
};

export type Membership = {
  company_id: string;
  role: "owner" | "member";
};

export type CompanyMember = {
  user_id: string;
  email: string;
  role: "owner" | "member";
};

export type CompanyCreateInput = {
  name: string;
  inn?: string;
  industry?: string;
  region?: string;
  city?: string;
  description?: string;
  revenue?: string;
};

export type Company = {
  id: string;
  name: string;
  inn: string;
  industry: string;
  region: string;
  description: string;
  group_id?: string;
  city?: string;
  client_type?: string;
  revenue?: string;
  okved_code?: string;
  okved_name?: string;
  products?: Record<string, string>;
  attributes?: Record<string, string>;
  created_at: string;
};

export type FeedItem = {
  event_id: string;
  company_id: string;
  title: string;
  summary: string;
  source?: string;
  url?: string;
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

export type FeedSort = "relevance" | "date";

export type FeedFilters = {
  sources?: string[];
  tags?: string[];
  sectors?: string[];
  query?: string;
  from?: string;
  to?: string;
  sort?: FeedSort;
};

export type Source = {
  source_id: string;
  title: string;
  url: string;
};

export type CitedClaim = {
  text: string;
  source_ids: string[];
  confidence?: "low" | "medium" | "high";
};

export type BankView = {
  used: boolean;
  text: string;
  source_ids: string[];
};

export type AIView = {
  text: string;
  source_ids: string[];
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
  facts?: CitedClaim[];
  hypotheses?: CitedClaim[];
  structured_risks?: CitedClaim[];
  structured_opportunities?: CitedClaim[];
  bank_view?: BankView;
  ai_view?: AIView;
  disclaimer?: string;
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
