import { ApiClient } from '@/lib/api';

describe('ApiClient', () => {
  let client: ApiClient;
  const mockFetch = global.fetch as jest.Mock;

  beforeEach(() => {
    client = new ApiClient('http://localhost:8080');
    mockFetch.mockClear();
  });

  describe('health', () => {
    it('should return health status when API is healthy', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: () => Promise.resolve({ status: 'healthy', database: 'ok', nats: 'ok' }),
      });

      const result = await client.health();

      expect(result).toEqual({ status: 'healthy', database: 'ok', nats: 'ok' });
      expect(mockFetch).toHaveBeenCalledWith(
        'http://localhost:8080/health',
        expect.objectContaining({
          headers: { 'Content-Type': 'application/json' },
        })
      );
    });

    it('should throw error when API returns error', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 500,
        json: () => Promise.resolve({ error: 'Internal server error' }),
      });

      await expect(client.health()).rejects.toThrow('Internal server error');
    });
  });

  describe('listRepos', () => {
    it('should return list of repositories', async () => {
      const mockRepos = [
        { id: '1', name: 'repo1', url: 'https://github.com/test/repo1' },
        { id: '2', name: 'repo2', url: 'https://github.com/test/repo2' },
      ];
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: () => Promise.resolve(mockRepos),
      });

      const result = await client.listRepos();

      expect(result).toEqual(mockRepos);
      expect(mockFetch).toHaveBeenCalledWith(
        'http://localhost:8080/api/v1/repos?limit=20&offset=0',
        expect.any(Object)
      );
    });

    it('should respect limit and offset parameters', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: () => Promise.resolve([]),
      });

      await client.listRepos(10, 5);

      expect(mockFetch).toHaveBeenCalledWith(
        'http://localhost:8080/api/v1/repos?limit=10&offset=5',
        expect.any(Object)
      );
    });
  });

  describe('getRepo', () => {
    it('should return repository by ID', async () => {
      const mockRepo = { id: '123', name: 'test-repo', url: 'https://github.com/test/repo' };
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: () => Promise.resolve(mockRepo),
      });

      const result = await client.getRepo('123');

      expect(result).toEqual(mockRepo);
      expect(mockFetch).toHaveBeenCalledWith(
        'http://localhost:8080/api/v1/repos/123',
        expect.any(Object)
      );
    });
  });

  describe('createRepo', () => {
    it('should create a new repository', async () => {
      const mockRepo = { id: '1', name: 'new-repo', url: 'https://github.com/test/new-repo' };
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 201,
        json: () => Promise.resolve(mockRepo),
      });

      const result = await client.createRepo('https://github.com/test/new-repo');

      expect(result).toEqual(mockRepo);
      expect(mockFetch).toHaveBeenCalledWith(
        'http://localhost:8080/api/v1/repos',
        expect.objectContaining({
          method: 'POST',
          body: JSON.stringify({ url: 'https://github.com/test/new-repo', branch: undefined }),
        })
      );
    });
  });

  describe('listJobs', () => {
    it('should return list of jobs', async () => {
      const mockJobs = [
        { id: '1', type: 'ingestion', status: 'completed' },
        { id: '2', type: 'generation', status: 'running' },
      ];
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: () => Promise.resolve(mockJobs),
      });

      const result = await client.listJobs();

      expect(result).toEqual(mockJobs);
    });

    it('should include filters in query string', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: () => Promise.resolve([]),
      });

      await client.listJobs({ status: 'running', type: 'generation', limit: 10 });

      expect(mockFetch).toHaveBeenCalledWith(
        'http://localhost:8080/api/v1/jobs?status=running&type=generation&limit=10',
        expect.any(Object)
      );
    });
  });

  describe('getCoverageSummary', () => {
    it('should return coverage summary', async () => {
      const mockSummary = {
        total_repos: 5,
        avg_coverage: 75.5,
        total_lines: 10000,
        total_covered: 7550,
        repos_above_80: 3,
        repos_below_50: 1,
        trend_direction: 'up',
        trend_delta: 2.5,
      };
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: () => Promise.resolve(mockSummary),
      });

      const result = await client.getCoverageSummary();

      expect(result).toEqual(mockSummary);
      expect(mockFetch).toHaveBeenCalledWith(
        'http://localhost:8080/api/v1/coverage/summary',
        expect.any(Object)
      );
    });
  });

  describe('listAPIKeys', () => {
    it('should return list of API keys', async () => {
      const mockKeys = [
        { id: '1', name: 'CI Key', key_prefix: 'qtest_abc', scopes: ['read:repos'] },
      ];
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: () => Promise.resolve(mockKeys),
      });

      const result = await client.listAPIKeys();

      expect(result).toEqual(mockKeys);
    });
  });

  describe('setSession', () => {
    it('should include authorization header when session is set', async () => {
      client.setSession('test-session-id');
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: () => Promise.resolve({}),
      });

      await client.health();

      expect(mockFetch).toHaveBeenCalledWith(
        expect.any(String),
        expect.objectContaining({
          headers: expect.objectContaining({
            'Authorization': 'Bearer test-session-id',
          }),
        })
      );
    });
  });

  describe('error handling', () => {
    it('should handle 204 No Content responses', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 204,
      });

      const result = await client.deleteRepo('123');

      expect(result).toBeUndefined();
    });

    it('should throw generic error when JSON parsing fails', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 500,
        json: () => Promise.reject(new Error('Invalid JSON')),
      });

      await expect(client.health()).rejects.toThrow('Unknown error');
    });
  });
});
