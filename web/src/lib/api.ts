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
}

// Export singleton instance
export const api = new ApiClient();

// Export class for custom instances
export { ApiClient };
