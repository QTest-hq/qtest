// QTest API Client

const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL || "http://192.168.1.131:8080";

export interface Repository {
  id: string;
  url: string;
  name: string;
  owner: string;
  default_branch: string;
  status: string;
  commit_sha?: string;
  created_at: string;
  updated_at: string;
}

export interface Job {
  id: string;
  type: string;
  status: string;
  priority: number;
  repository_id?: string;
  generation_run_id?: string;
  parent_job_id?: string;
  error_message?: string;
  retry_count: number;
  max_retries: number;
  created_at: string;
  updated_at: string;
  started_at?: string;
  completed_at?: string;
}

export interface MutationRun {
  id: string;
  status: string;
  source_file: string;
  test_file: string;
  repository_id?: string;
  generation_run_id?: string;
  result?: MutationResult;
  created_at: string;
  completed_at?: string;
}

export interface MutationResult {
  total: number;
  killed: number;
  survived: number;
  timeout: number;
  score: number;
  quality: string;
}

export interface User {
  login: string;
  id: number;
  avatar_url: string;
  name?: string;
  email?: string;
}

export interface HealthStatus {
  status: string;
  database?: string;
  nats?: string;
}

export interface CoverageSummary {
  total_repos: number;
  avg_coverage: number;
  total_lines: number;
  total_covered: number;
  repos_above_80: number;
  repos_below_50: number;
  trend_direction: string;
  trend_delta: number;
}

export interface CoverageSnapshot {
  id: string;
  repository_id: string;
  commit_sha?: string;
  branch?: string;
  language: string;
  total_lines: number;
  covered_lines: number;
  coverage_percent: number;
  lines_delta: number;
  coverage_delta: number;
  created_at: string;
}

export interface CoverageTrend {
  date: string;
  avg_coverage: number;
  snapshot_count: number;
}

// IRSpec types - Universal Intermediate Representation for tests
export interface IRVariable {
  name: string;
  value: unknown;
  type: "int" | "float" | "string" | "bool" | "null" | "array" | "object";
}

export interface IRAction {
  call: string;
  args?: string[];
}

export interface IRAssertion {
  type: "equals" | "not_equals" | "contains" | "greater_than" | "less_than" | "throws" | "truthy" | "falsy" | "nil" | "not_nil" | "length";
  actual: string;
  expected?: unknown;
  message?: string;
}

export interface IRTestCase {
  name: string;
  description?: string;
  given: IRVariable[];
  when: IRAction;
  then: IRAssertion[];
  tags?: string[];
}

export interface IRTestSuite {
  function_name: string;
  description?: string;
  tests: IRTestCase[];
}

export interface TestMetadata {
  irspec?: IRTestSuite;
  test_specs?: unknown;
  irspec_mode?: boolean;
}

export interface GeneratedTest {
  id: string;
  run_id: string;
  name: string;
  type: string;
  target_file: string;
  target_function?: string;
  dsl?: unknown;
  generated_code?: string;
  framework?: string;
  status: string;
  rejection_reason?: string;
  mutation_score?: number;
  metadata?: TestMetadata;
  created_at: string;
  updated_at: string;
}

class ApiClient {
  private baseUrl: string;
  private sessionId?: string;

  constructor(baseUrl: string = API_BASE_URL) {
    this.baseUrl = baseUrl;
  }

  setSession(sessionId: string) {
    this.sessionId = sessionId;
  }

  private async request<T>(
    endpoint: string,
    options: RequestInit = {}
  ): Promise<T> {
    const headers: Record<string, string> = {
      "Content-Type": "application/json",
    };

    // Merge existing headers
    if (options.headers) {
      const existingHeaders = options.headers as Record<string, string>;
      Object.assign(headers, existingHeaders);
    }

    if (this.sessionId) {
      headers["Authorization"] = `Bearer ${this.sessionId}`;
    }

    const response = await fetch(`${this.baseUrl}${endpoint}`, {
      ...options,
      headers,
    });

    if (!response.ok) {
      const error = await response.json().catch(() => ({ error: "Unknown error" }));
      throw new Error(error.error || `HTTP ${response.status}`);
    }

    if (response.status === 204) {
      return {} as T;
    }

    return response.json();
  }

  // Health endpoints
  async health(): Promise<HealthStatus> {
    return this.request<HealthStatus>("/health");
  }

  async ready(): Promise<HealthStatus> {
    return this.request<HealthStatus>("/ready");
  }

  // Auth endpoints
  getLoginUrl(): string {
    return `${this.baseUrl}/auth/login`;
  }

  async me(): Promise<{ user: User; session_id: string; expires_at: string }> {
    return this.request("/api/v1/auth/me");
  }

  async refresh(): Promise<{ session_id: string; expires_at: string }> {
    return this.request("/api/v1/auth/refresh", { method: "POST" });
  }

  async logout(): Promise<void> {
    await this.request("/auth/logout", { method: "POST" });
    this.sessionId = undefined;
  }

  async userRepos(): Promise<Repository[]> {
    return this.request("/api/v1/auth/repos");
  }

  // Repository endpoints
  async listRepos(limit = 20, offset = 0): Promise<Repository[]> {
    return this.request(`/api/v1/repos?limit=${limit}&offset=${offset}`);
  }

  async getRepo(id: string): Promise<Repository> {
    return this.request(`/api/v1/repos/${id}`);
  }

  async createRepo(url: string, branch?: string): Promise<Repository> {
    return this.request("/api/v1/repos", {
      method: "POST",
      body: JSON.stringify({ url, branch }),
    });
  }

  async deleteRepo(id: string): Promise<void> {
    await this.request(`/api/v1/repos/${id}`, { method: "DELETE" });
  }

  // Job endpoints
  async listJobs(params?: {
    status?: string;
    type?: string;
    limit?: number;
  }): Promise<Job[]> {
    const searchParams = new URLSearchParams();
    if (params?.status) searchParams.set("status", params.status);
    if (params?.type) searchParams.set("type", params.type);
    if (params?.limit) searchParams.set("limit", params.limit.toString());
    const query = searchParams.toString();
    return this.request(`/api/v1/jobs${query ? `?${query}` : ""}`);
  }

  async getJob(id: string): Promise<Job> {
    return this.request(`/api/v1/jobs/${id}`);
  }

  async startPipeline(params: {
    repository_url: string;
    branch?: string;
    max_tests?: number;
    llm_tier?: number;
  }): Promise<Job> {
    return this.request("/api/v1/jobs/pipeline", {
      method: "POST",
      body: JSON.stringify(params),
    });
  }

  async cancelJob(id: string): Promise<void> {
    await this.request(`/api/v1/jobs/${id}/cancel`, { method: "POST" });
  }

  async retryJob(id: string): Promise<Job> {
    return this.request(`/api/v1/jobs/${id}/retry`, { method: "POST" });
  }

  async getJobsByRepo(repoId: string, limit = 20): Promise<Job[]> {
    return this.request(`/api/v1/jobs?repository_id=${repoId}&limit=${limit}`);
  }

  async getJobChildren(parentId: string): Promise<Job[]> {
    return this.request(`/api/v1/jobs?parent_id=${parentId}`);
  }

  // Mutation endpoints
  async createMutationRun(params: {
    source_file_path: string;
    test_file_path: string;
    repository_id?: string;
    generation_run_id?: string;
    mode?: "fast" | "thorough";
  }): Promise<MutationRun> {
    return this.request("/api/v1/mutation", {
      method: "POST",
      body: JSON.stringify(params),
    });
  }

  async listMutationRuns(params?: {
    status?: string;
    limit?: number;
  }): Promise<MutationRun[]> {
    const searchParams = new URLSearchParams();
    if (params?.status) searchParams.set("status", params.status);
    if (params?.limit) searchParams.set("limit", params.limit.toString());
    const query = searchParams.toString();
    return this.request(`/api/v1/mutation${query ? `?${query}` : ""}`);
  }

  async getMutationRun(id: string): Promise<MutationRun> {
    return this.request(`/api/v1/mutation/${id}`);
  }

  // Generated Tests endpoints
  async getTest(id: string): Promise<GeneratedTest> {
    return this.request(`/api/v1/tests/${id}`);
  }

  async listTests(params?: {
    run_id?: string;
    status?: string;
    limit?: number;
  }): Promise<GeneratedTest[]> {
    const searchParams = new URLSearchParams();
    if (params?.run_id) searchParams.set("run_id", params.run_id);
    if (params?.status) searchParams.set("status", params.status);
    if (params?.limit) searchParams.set("limit", params.limit.toString());
    const query = searchParams.toString();
    return this.request(`/api/v1/tests${query ? `?${query}` : ""}`);
  }

  async acceptTest(id: string): Promise<void> {
    await this.request(`/api/v1/tests/${id}/accept`, { method: "PUT" });
  }

  async rejectTest(id: string, reason?: string): Promise<void> {
    await this.request(`/api/v1/tests/${id}/reject`, {
      method: "PUT",
      body: JSON.stringify({ reason }),
    });
  }

  // Coverage endpoints
  async getCoverageSummary(): Promise<CoverageSummary> {
    return this.request("/api/v1/coverage/summary");
  }

  async listCoverageSnapshots(repoId?: string, limit = 50): Promise<CoverageSnapshot[]> {
    const params = new URLSearchParams();
    if (repoId) params.set("repository_id", repoId);
    if (limit) params.set("limit", limit.toString());
    const query = params.toString();
    return this.request(`/api/v1/coverage/snapshots${query ? `?${query}` : ""}`);
  }

  async getCoverageTrend(repoId: string, days = 30): Promise<CoverageTrend[]> {
    return this.request(`/api/v1/coverage/repos/${repoId}/trend?days=${days}`);
  }

  // API Key endpoints
  async listAPIKeys(organizationId?: string): Promise<APIKey[]> {
    const params = organizationId ? `?organization_id=${organizationId}` : "";
    return this.request(`/api/v1/api-keys${params}`);
  }

  async createAPIKey(req: CreateAPIKeyRequest): Promise<APIKey> {
    return this.request("/api/v1/api-keys", {
      method: "POST",
      body: JSON.stringify(req),
    });
  }

  async revokeAPIKey(keyId: string): Promise<void> {
    await this.request(`/api/v1/api-keys/${keyId}`, { method: "DELETE" });
  }

  // Organization endpoints
  async listOrganizations(): Promise<OrganizationWithRole[]> {
    return this.request("/api/v1/organizations");
  }

  async getOrganization(orgId: string): Promise<{ organization: Organization; role: MemberRole }> {
    return this.request(`/api/v1/organizations/${orgId}`);
  }

  async createOrganization(req: CreateOrganizationRequest): Promise<Organization> {
    return this.request("/api/v1/organizations", {
      method: "POST",
      body: JSON.stringify(req),
    });
  }

  async updateOrganization(orgId: string, updates: { name?: string; description?: string }): Promise<Organization> {
    return this.request(`/api/v1/organizations/${orgId}`, {
      method: "PATCH",
      body: JSON.stringify(updates),
    });
  }

  async deleteOrganization(orgId: string): Promise<void> {
    await this.request(`/api/v1/organizations/${orgId}`, { method: "DELETE" });
  }

  // Organization members endpoints
  async listMembers(orgId: string): Promise<OrganizationMember[]> {
    return this.request(`/api/v1/organizations/${orgId}/members`);
  }

  async addMember(orgId: string, req: AddMemberRequest): Promise<void> {
    await this.request(`/api/v1/organizations/${orgId}/members`, {
      method: "POST",
      body: JSON.stringify(req),
    });
  }

  async updateMemberRole(orgId: string, userId: string, role: MemberRole): Promise<void> {
    await this.request(`/api/v1/organizations/${orgId}/members/${userId}`, {
      method: "PATCH",
      body: JSON.stringify({ role }),
    });
  }

  async removeMember(orgId: string, userId: string): Promise<void> {
    await this.request(`/api/v1/organizations/${orgId}/members/${userId}`, { method: "DELETE" });
  }

  // Team Stats (aggregated from repos/jobs in org)
  async getOrgStats(orgId: string): Promise<TeamStats> {
    // This endpoint may need to be added to the backend
    // For now, we'll aggregate from existing endpoints
    return this.request(`/api/v1/organizations/${orgId}/stats`);
  }

  async getOrgRepos(orgId: string, limit = 20): Promise<Repository[]> {
    return this.request(`/api/v1/organizations/${orgId}/repos?limit=${limit}`);
  }

  async getOrgJobs(orgId: string, limit = 20): Promise<Job[]> {
    return this.request(`/api/v1/organizations/${orgId}/jobs?limit=${limit}`);
  }

  // Webhook endpoints
  async listWebhooks(orgId: string, limit = 20, offset = 0): Promise<Webhook[]> {
    return this.request(`/api/v1/organizations/${orgId}/webhooks?limit=${limit}&offset=${offset}`);
  }

  async createWebhook(orgId: string, req: CreateWebhookRequest): Promise<WebhookWithSecret> {
    return this.request(`/api/v1/organizations/${orgId}/webhooks`, {
      method: "POST",
      body: JSON.stringify(req),
    });
  }

  async getWebhook(orgId: string, webhookId: string): Promise<Webhook> {
    return this.request(`/api/v1/organizations/${orgId}/webhooks/${webhookId}`);
  }

  async updateWebhook(orgId: string, webhookId: string, updates: UpdateWebhookRequest): Promise<Webhook> {
    return this.request(`/api/v1/organizations/${orgId}/webhooks/${webhookId}`, {
      method: "PATCH",
      body: JSON.stringify(updates),
    });
  }

  async deleteWebhook(orgId: string, webhookId: string): Promise<void> {
    await this.request(`/api/v1/organizations/${orgId}/webhooks/${webhookId}`, { method: "DELETE" });
  }

  async listWebhookDeliveries(orgId: string, webhookId: string, limit = 20): Promise<WebhookDelivery[]> {
    return this.request(`/api/v1/organizations/${orgId}/webhooks/${webhookId}/deliveries?limit=${limit}`);
  }

  async sendTestWebhook(orgId: string, webhookId: string): Promise<{ status: string; message: string }> {
    return this.request(`/api/v1/organizations/${orgId}/webhooks/${webhookId}/test`, { method: "POST" });
  }

  async retryWebhookDelivery(orgId: string, webhookId: string, deliveryId: string): Promise<void> {
    await this.request(`/api/v1/organizations/${orgId}/webhooks/${webhookId}/deliveries/${deliveryId}/retry`, {
      method: "POST",
    });
  }

  async regenerateWebhookSecret(orgId: string, webhookId: string): Promise<{ secret: string }> {
    return this.request(`/api/v1/organizations/${orgId}/webhooks/${webhookId}/secret`, {
      method: "POST",
    });
  }
}

// Export singleton instance
export const api = new ApiClient();

// API Key types
export interface APIKey {
  id: string;
  organization_id: string;
  name: string;
  key_prefix: string;
  scopes: string[];
  expires_at?: string;
  last_used_at?: string;
  created_at: string;
  secret?: string; // Only returned on creation
}

export interface CreateAPIKeyRequest {
  name: string;
  scopes: string[];
  organization_id?: string;
  expires_in_days?: number;
}

// Organization types
export type MemberRole = "owner" | "admin" | "member" | "viewer";

export interface Organization {
  id: string;
  name: string;
  slug: string;
  description?: string;
  owner_id: string;
  github_org_id?: number;
  settings?: Record<string, unknown>;
  is_personal: boolean;
  created_at: string;
  updated_at: string;
}

export interface OrganizationWithRole extends Organization {
  role: MemberRole;
}

export interface OrganizationMember {
  id: string;
  organization_id: string;
  user_id: string;
  role: MemberRole;
  invited_by?: string;
  joined_at: string;
  created_at: string;
  github_login: string;
  name?: string;
  avatar_url?: string;
}

export interface CreateOrganizationRequest {
  name: string;
  slug: string;
  description?: string;
}

export interface AddMemberRequest {
  user_id: string;
  role: MemberRole;
}

// Team Stats types
export interface TeamStats {
  total_repos: number;
  total_tests_generated: number;
  avg_coverage: number;
  total_jobs: number;
  jobs_this_week: number;
  active_members: number;
}

export interface TeamActivity {
  id: string;
  type: string;
  actor_name: string;
  actor_avatar?: string;
  description: string;
  repo_name?: string;
  created_at: string;
}

// Webhook types
export interface Webhook {
  id: string;
  organization_id: string;
  name: string;
  url: string;
  events: string[];
  is_active: boolean;
  max_retries: number;
  timeout_seconds: number;
  description?: string;
  headers?: Record<string, string>;
  created_at: string;
  updated_at: string;
  last_triggered_at?: string;
}

export interface WebhookWithSecret extends Webhook {
  secret: string; // Only returned on creation
}

export interface WebhookDelivery {
  id: string;
  webhook_id: string;
  event_type: string;
  event_id: string;
  status: "pending" | "success" | "failed" | "retrying";
  attempt_count: number;
  response_status?: number;
  response_body?: string;
  error_message?: string;
  request_body?: string;
  created_at: string;
  delivered_at?: string;
  duration_ms?: number;
}

export interface CreateWebhookRequest {
  name: string;
  url: string;
  events: string[];
  description?: string;
  headers?: Record<string, string>;
}

export interface UpdateWebhookRequest {
  name?: string;
  url?: string;
  events?: string[];
  description?: string;
  headers?: Record<string, string>;
  is_active?: boolean;
}

// Supported webhook event types
export const WEBHOOK_EVENT_TYPES = [
  "job.completed",
  "job.failed",
  "run.started",
  "run.completed",
  "tests.generated",
  "tests.validated",
  "mutation.completed",
] as const;

export type WebhookEventType = typeof WEBHOOK_EVENT_TYPES[number];

// Export class for custom instances
export { ApiClient };
