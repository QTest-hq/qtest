"use client";

import { useState, useEffect } from "react";
import Sidebar from "@/components/Sidebar";
import { useAuth } from "@/contexts/AuthContext";
import { api, APIKey, CreateAPIKeyRequest } from "@/lib/api";

const AVAILABLE_SCOPES = [
  { value: "read:repos", label: "Read Repositories", description: "View repositories" },
  { value: "write:repos", label: "Write Repositories", description: "Create and modify repositories" },
  { value: "read:runs", label: "Read Runs", description: "View generation runs" },
  { value: "write:runs", label: "Write Runs", description: "Create generation runs" },
  { value: "read:tests", label: "Read Tests", description: "View generated tests" },
  { value: "write:tests", label: "Write Tests", description: "Accept/reject tests" },
  { value: "read:jobs", label: "Read Jobs", description: "View job queue" },
  { value: "write:jobs", label: "Write Jobs", description: "Create and cancel jobs" },
  { value: "read:mutation", label: "Read Mutation", description: "View mutation results" },
];

export default function SettingsPage() {
  const { user, loading: authLoading, logout } = useAuth();
  const [apiStatus, setApiStatus] = useState<"checking" | "online" | "offline">("checking");
  const [llmTier, setLlmTier] = useState(1);
  const [maxTests, setMaxTests] = useState(10);
  const [apiKeys, setApiKeys] = useState<APIKey[]>([]);
  const [keysLoading, setKeysLoading] = useState(false);
  const [showCreateModal, setShowCreateModal] = useState(false);
  const [newKeyName, setNewKeyName] = useState("");
  const [selectedScopes, setSelectedScopes] = useState<string[]>(["read:repos", "read:runs", "read:tests"]);
  const [expiresInDays, setExpiresInDays] = useState<number | undefined>(90);
  const [createdKey, setCreatedKey] = useState<APIKey | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    checkApiHealth();
    if (user) {
      loadAPIKeys();
    }
  }, [user]);

  async function checkApiHealth() {
    try {
      await api.health();
      setApiStatus("online");
    } catch {
      setApiStatus("offline");
    }
  }

  async function loadAPIKeys() {
    try {
      setKeysLoading(true);
      const keys = await api.listAPIKeys();
      setApiKeys(keys);
    } catch (err) {
      console.error("Failed to load API keys:", err);
    } finally {
      setKeysLoading(false);
    }
  }

  async function createAPIKey() {
    if (!newKeyName.trim()) {
      setError("Please enter a name for the API key");
      return;
    }
    if (selectedScopes.length === 0) {
      setError("Please select at least one scope");
      return;
    }

    try {
      setError(null);
      const req: CreateAPIKeyRequest = {
        name: newKeyName.trim(),
        scopes: selectedScopes,
        expires_in_days: expiresInDays,
      };
      const key = await api.createAPIKey(req);
      setCreatedKey(key);
      setApiKeys([key, ...apiKeys]);
      setNewKeyName("");
      setSelectedScopes(["read:repos", "read:runs", "read:tests"]);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to create API key");
    }
  }

  async function revokeKey(keyId: string) {
    if (!confirm("Are you sure you want to revoke this API key? This action cannot be undone.")) {
      return;
    }

    try {
      await api.revokeAPIKey(keyId);
      setApiKeys(apiKeys.filter(k => k.id !== keyId));
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to revoke API key");
    }
  }

  function toggleScope(scope: string) {
    if (selectedScopes.includes(scope)) {
      setSelectedScopes(selectedScopes.filter(s => s !== scope));
    } else {
      setSelectedScopes([...selectedScopes, scope]);
    }
  }

  function closeCreateModal() {
    setShowCreateModal(false);
    setCreatedKey(null);
    setError(null);
  }

  return (
    <div className="flex h-screen">
      <Sidebar />

      <main className="flex-1 overflow-y-auto">
        {/* Header */}
        <div className="border-b border-gray-200 bg-white px-8 py-6 dark:border-gray-700 dark:bg-gray-800">
          <h1 className="text-2xl font-semibold text-gray-900 dark:text-white">Settings</h1>
          <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
            Manage your account and configure QTest
          </p>
        </div>

        <div className="p-8 max-w-4xl">
          {/* Profile Section */}
          <section className="mb-6">
            <h2 className="text-lg font-medium text-gray-900 dark:text-white mb-4">Profile</h2>
            <div className="rounded-lg bg-white shadow-sm ring-1 ring-gray-200 dark:bg-gray-800 dark:ring-gray-700">
              {authLoading ? (
                <div className="p-6 flex items-center">
                  <div className="h-16 w-16 rounded-full bg-gray-200 dark:bg-gray-700 animate-pulse" />
                  <div className="ml-4 space-y-2">
                    <div className="h-5 w-32 bg-gray-200 dark:bg-gray-700 rounded animate-pulse" />
                    <div className="h-4 w-48 bg-gray-200 dark:bg-gray-700 rounded animate-pulse" />
                  </div>
                </div>
              ) : user ? (
                <div className="p-6">
                  <div className="flex items-center justify-between">
                    <div className="flex items-center">
                      {user.avatar_url ? (
                        <img src={user.avatar_url} alt={user.login} className="h-16 w-16 rounded-full" />
                      ) : (
                        <div className="h-16 w-16 rounded-full bg-indigo-600 flex items-center justify-center">
                          <span className="text-xl font-medium text-white">
                            {user.login.slice(0, 2).toUpperCase()}
                          </span>
                        </div>
                      )}
                      <div className="ml-4">
                        <p className="text-lg font-medium text-gray-900 dark:text-white">
                          {user.name || user.login}
                        </p>
                        <p className="text-sm text-gray-500 dark:text-gray-400">@{user.login}</p>
                        {user.email && (
                          <p className="text-sm text-gray-500 dark:text-gray-400">{user.email}</p>
                        )}
                      </div>
                    </div>
                    <button
                      onClick={logout}
                      className="rounded-lg bg-gray-100 px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-200 dark:bg-gray-700 dark:text-gray-300 dark:hover:bg-gray-600"
                    >
                      Sign out
                    </button>
                  </div>
                </div>
              ) : (
                <div className="p-6 text-center">
                  <p className="text-gray-500 dark:text-gray-400">Sign in to view your profile</p>
                </div>
              )}
            </div>
          </section>

          {/* API Keys Section */}
          <section className="mb-6">
            <div className="flex items-center justify-between mb-4">
              <h2 className="text-lg font-medium text-gray-900 dark:text-white">API Keys</h2>
              {user && (
                <button
                  onClick={() => setShowCreateModal(true)}
                  className="rounded-lg bg-indigo-600 px-4 py-2 text-sm font-medium text-white hover:bg-indigo-500"
                >
                  Create API Key
                </button>
              )}
            </div>
            <div className="rounded-lg bg-white shadow-sm ring-1 ring-gray-200 dark:bg-gray-800 dark:ring-gray-700">
              {!user ? (
                <div className="p-6 text-center">
                  <p className="text-gray-500 dark:text-gray-400">Sign in to manage API keys</p>
                </div>
              ) : keysLoading ? (
                <div className="p-6 flex items-center justify-center">
                  <div className="h-6 w-6 animate-spin rounded-full border-2 border-indigo-600 border-t-transparent" />
                  <span className="ml-2 text-gray-500">Loading API keys...</span>
                </div>
              ) : apiKeys.length === 0 ? (
                <div className="p-6 text-center">
                  <svg className="mx-auto h-12 w-12 text-gray-400" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor">
                    <path strokeLinecap="round" strokeLinejoin="round" d="M15.75 5.25a3 3 0 0 1 3 3m3 0a6 6 0 0 1-7.029 5.912c-.563-.097-1.159.026-1.563.43L10.5 17.25H8.25v2.25H6v2.25H2.25v-2.818c0-.597.237-1.17.659-1.591l6.499-6.499c.404-.404.527-1 .43-1.563A6 6 0 1 1 21.75 8.25Z" />
                  </svg>
                  <h3 className="mt-4 text-lg font-medium text-gray-900 dark:text-white">No API keys</h3>
                  <p className="mt-2 text-sm text-gray-500 dark:text-gray-400">
                    Create an API key to access QTest programmatically
                  </p>
                </div>
              ) : (
                <div className="overflow-x-auto">
                  <table className="min-w-full divide-y divide-gray-200 dark:divide-gray-700">
                    <thead className="bg-gray-50 dark:bg-gray-900">
                      <tr>
                        <th className="px-6 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400">Name</th>
                        <th className="px-6 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400">Key</th>
                        <th className="px-6 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400">Scopes</th>
                        <th className="px-6 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400">Created</th>
                        <th className="px-6 py-3 text-right text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400">Actions</th>
                      </tr>
                    </thead>
                    <tbody className="divide-y divide-gray-200 dark:divide-gray-700">
                      {apiKeys.map((key) => (
                        <tr key={key.id} className="hover:bg-gray-50 dark:hover:bg-gray-700">
                          <td className="px-6 py-4 whitespace-nowrap text-sm font-medium text-gray-900 dark:text-white">{key.name}</td>
                          <td className="px-6 py-4 whitespace-nowrap">
                            <code className="text-sm text-gray-500 dark:text-gray-400 bg-gray-100 dark:bg-gray-900 px-2 py-1 rounded">{key.key_prefix}...</code>
                          </td>
                          <td className="px-6 py-4">
                            <div className="flex flex-wrap gap-1">
                              {key.scopes.slice(0, 2).map((scope) => (
                                <span key={scope} className="inline-flex items-center rounded-full bg-indigo-100 dark:bg-indigo-900/30 px-2 py-0.5 text-xs font-medium text-indigo-700 dark:text-indigo-300">{scope}</span>
                              ))}
                              {key.scopes.length > 2 && <span className="text-xs text-gray-500">+{key.scopes.length - 2}</span>}
                            </div>
                          </td>
                          <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500 dark:text-gray-400">{new Date(key.created_at).toLocaleDateString()}</td>
                          <td className="px-6 py-4 whitespace-nowrap text-right">
                            <button onClick={() => revokeKey(key.id)} className="text-sm text-red-600 hover:text-red-500">Revoke</button>
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              )}
            </div>
          </section>

          {/* API Status */}
          <div className="mb-6 rounded-lg bg-white p-6 shadow-sm ring-1 ring-gray-200 dark:bg-gray-800 dark:ring-gray-700">
            <h2 className="text-lg font-medium text-gray-900 dark:text-white">API Connection</h2>
            <div className="mt-4 flex items-center">
              <div className={`h-3 w-3 rounded-full ${apiStatus === "checking" ? "bg-yellow-400 animate-pulse" : apiStatus === "online" ? "bg-green-400" : "bg-red-400"}`} />
              <span className="ml-3 text-sm text-gray-700 dark:text-gray-300">
                {apiStatus === "checking" ? "Checking connection..." : apiStatus === "online" ? "Connected to API server" : "API server unavailable"}
              </span>
              <button onClick={checkApiHealth} className="ml-auto text-sm text-indigo-600 hover:text-indigo-700 dark:text-indigo-400">Refresh</button>
            </div>
            <p className="mt-2 text-xs text-gray-500 dark:text-gray-400">API endpoint: {process.env.NEXT_PUBLIC_API_URL || "http://192.168.1.131:8080"}</p>
          </div>

          {/* Generation Defaults */}
          <div className="mb-6 rounded-lg bg-white p-6 shadow-sm ring-1 ring-gray-200 dark:bg-gray-800 dark:ring-gray-700">
            <h2 className="text-lg font-medium text-gray-900 dark:text-white">Generation Defaults</h2>
            <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">Default settings for test generation pipelines</p>
            <div className="mt-6 space-y-6">
              <div>
                <label className="block text-sm font-medium text-gray-700 dark:text-gray-300">Default LLM Tier</label>
                <select value={llmTier} onChange={(e) => setLlmTier(parseInt(e.target.value))} className="mt-1 block w-full rounded-lg border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 dark:border-gray-600 dark:bg-gray-700 dark:text-white px-4 py-2">
                  <option value={1}>Tier 1: Fast (qwen2.5-coder:7b)</option>
                  <option value={2}>Tier 2: Balanced (deepseek-coder-v2:16b)</option>
                  <option value={3}>Tier 3: Thorough (deepseek-coder-v2:16b)</option>
                </select>
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700 dark:text-gray-300">Default Max Tests per File</label>
                <input type="number" value={maxTests} onChange={(e) => setMaxTests(parseInt(e.target.value) || 10)} min={1} max={50} className="mt-1 block w-32 rounded-lg border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 dark:border-gray-600 dark:bg-gray-700 dark:text-white px-4 py-2" />
              </div>
            </div>
          </div>

          {/* About */}
          <div className="rounded-lg bg-white p-6 shadow-sm ring-1 ring-gray-200 dark:bg-gray-800 dark:ring-gray-700">
            <h2 className="text-lg font-medium text-gray-900 dark:text-white">About QTest</h2>
            <div className="mt-4 space-y-2 text-sm text-gray-600 dark:text-gray-400">
              <p><span className="font-medium text-gray-900 dark:text-white">Version:</span> 0.1.0</p>
              <p><span className="font-medium text-gray-900 dark:text-white">License:</span> MIT</p>
              <p className="pt-2">QTest is an AI-powered test generation platform that transforms any repository into a comprehensive test suite using LLMs.</p>
            </div>
            <div className="mt-4">
              <a href="https://github.com/QTest-hq/qtest" target="_blank" rel="noopener noreferrer" className="inline-flex items-center text-sm text-indigo-600 hover:text-indigo-700 dark:text-indigo-400">
                <svg className="h-4 w-4 mr-1" fill="currentColor" viewBox="0 0 24 24">
                  <path fillRule="evenodd" d="M12 2C6.477 2 2 6.484 2 12.017c0 4.425 2.865 8.18 6.839 9.504.5.092.682-.217.682-.483 0-.237-.008-.868-.013-1.703-2.782.605-3.369-1.343-3.369-1.343-.454-1.158-1.11-1.466-1.11-1.466-.908-.62.069-.608.069-.608 1.003.07 1.531 1.032 1.531 1.032.892 1.53 2.341 1.088 2.91.832.092-.647.35-1.088.636-1.338-2.22-.253-4.555-1.113-4.555-4.951 0-1.093.39-1.988 1.029-2.688-.103-.253-.446-1.272.098-2.65 0 0 .84-.27 2.75 1.026A9.564 9.564 0 0112 6.844c.85.004 1.705.115 2.504.337 1.909-1.296 2.747-1.027 2.747-1.027.546 1.379.202 2.398.1 2.651.64.7 1.028 1.595 1.028 2.688 0 3.848-2.339 4.695-4.566 4.943.359.309.678.92.678 1.855 0 1.338-.012 2.419-.012 2.747 0 .268.18.58.688.482A10.019 10.019 0 0022 12.017C22 6.484 17.522 2 12 2z" clipRule="evenodd" />
                </svg>
                View on GitHub
              </a>
            </div>
          </div>
        </div>
      </main>

      {/* Create API Key Modal */}
      {showCreateModal && (
        <div className="fixed inset-0 z-50 overflow-y-auto">
          <div className="flex min-h-screen items-end justify-center px-4 pb-20 pt-4 text-center sm:block sm:p-0">
            <div className="fixed inset-0 bg-gray-500 bg-opacity-75 transition-opacity" onClick={closeCreateModal} />
            <span className="hidden sm:inline-block sm:h-screen sm:align-middle">&#8203;</span>
            <div className="relative inline-block transform overflow-hidden rounded-lg bg-white dark:bg-gray-800 px-4 pb-4 pt-5 text-left align-bottom shadow-xl transition-all sm:my-8 sm:w-full sm:max-w-lg sm:p-6 sm:align-middle">
              {createdKey ? (
                <>
                  <div>
                    <div className="mx-auto flex h-12 w-12 items-center justify-center rounded-full bg-green-100 dark:bg-green-900/30">
                      <svg className="h-6 w-6 text-green-600 dark:text-green-400" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor">
                        <path strokeLinecap="round" strokeLinejoin="round" d="M4.5 12.75l6 6 9-13.5" />
                      </svg>
                    </div>
                    <div className="mt-3 text-center sm:mt-5">
                      <h3 className="text-lg font-medium text-gray-900 dark:text-white">API Key Created</h3>
                      <div className="mt-2">
                        <p className="text-sm text-gray-500 dark:text-gray-400 mb-4">Copy your API key now. You won&apos;t be able to see it again!</p>
                        <div className="bg-gray-100 dark:bg-gray-900 rounded-lg p-4">
                          <code className="text-sm break-all text-gray-900 dark:text-white">{createdKey.secret}</code>
                        </div>
                        <button onClick={() => navigator.clipboard.writeText(createdKey.secret || "")} className="mt-2 text-sm text-indigo-600 hover:text-indigo-500">Copy to clipboard</button>
                      </div>
                    </div>
                  </div>
                  <div className="mt-5 sm:mt-6">
                    <button type="button" className="inline-flex w-full justify-center rounded-lg bg-indigo-600 px-3 py-2 text-sm font-semibold text-white shadow-sm hover:bg-indigo-500" onClick={closeCreateModal}>Done</button>
                  </div>
                </>
              ) : (
                <>
                  <div>
                    <h3 className="text-lg font-medium text-gray-900 dark:text-white">Create API Key</h3>
                    <div className="mt-4 space-y-4">
                      {error && (
                        <div className="rounded-lg bg-red-50 dark:bg-red-900/20 p-3">
                          <p className="text-sm text-red-800 dark:text-red-200">{error}</p>
                        </div>
                      )}
                      <div>
                        <label htmlFor="keyName" className="block text-sm font-medium text-gray-700 dark:text-gray-300">Name</label>
                        <input type="text" id="keyName" value={newKeyName} onChange={(e) => setNewKeyName(e.target.value)} placeholder="e.g., CI/CD Pipeline" className="mt-1 block w-full rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 px-3 py-2 text-sm text-gray-900 dark:text-white placeholder-gray-500 focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500" />
                      </div>
                      <div>
                        <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">Scopes</label>
                        <div className="space-y-2 max-h-48 overflow-y-auto">
                          {AVAILABLE_SCOPES.map((scope) => (
                            <label key={scope.value} className="flex items-start">
                              <input type="checkbox" checked={selectedScopes.includes(scope.value)} onChange={() => toggleScope(scope.value)} className="mt-1 h-4 w-4 rounded border-gray-300 text-indigo-600 focus:ring-indigo-500" />
                              <span className="ml-2">
                                <span className="text-sm font-medium text-gray-900 dark:text-white">{scope.label}</span>
                                <p className="text-xs text-gray-500 dark:text-gray-400">{scope.description}</p>
                              </span>
                            </label>
                          ))}
                        </div>
                      </div>
                      <div>
                        <label htmlFor="expiration" className="block text-sm font-medium text-gray-700 dark:text-gray-300">Expiration</label>
                        <select id="expiration" value={expiresInDays || ""} onChange={(e) => setExpiresInDays(e.target.value ? parseInt(e.target.value) : undefined)} className="mt-1 block w-full rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 px-3 py-2 text-sm text-gray-900 dark:text-white focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500">
                          <option value="">Never expires</option>
                          <option value="30">30 days</option>
                          <option value="60">60 days</option>
                          <option value="90">90 days</option>
                          <option value="180">180 days</option>
                          <option value="365">1 year</option>
                        </select>
                      </div>
                    </div>
                  </div>
                  <div className="mt-5 sm:mt-6 sm:grid sm:grid-flow-row-dense sm:grid-cols-2 sm:gap-3">
                    <button type="button" className="inline-flex w-full justify-center rounded-lg bg-indigo-600 px-3 py-2 text-sm font-semibold text-white shadow-sm hover:bg-indigo-500 sm:col-start-2" onClick={createAPIKey}>Create</button>
                    <button type="button" className="mt-3 inline-flex w-full justify-center rounded-lg bg-white dark:bg-gray-700 px-3 py-2 text-sm font-semibold text-gray-900 dark:text-white shadow-sm ring-1 ring-inset ring-gray-300 dark:ring-gray-600 hover:bg-gray-50 dark:hover:bg-gray-600 sm:col-start-1 sm:mt-0" onClick={closeCreateModal}>Cancel</button>
                  </div>
                </>
              )}
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
