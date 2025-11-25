"use client";

import { useState, useEffect, useCallback } from "react";
import { useAuth } from "@/contexts/AuthContext";
import {
  api,
  OrganizationWithRole,
  Webhook,
  WebhookWithSecret,
  WebhookDelivery,
  CreateWebhookRequest,
  WEBHOOK_EVENT_TYPES,
} from "@/lib/api";

export default function WebhooksPage() {
  const { user, loading: authLoading } = useAuth();
  const [organizations, setOrganizations] = useState<OrganizationWithRole[]>([]);
  const [selectedOrg, setSelectedOrg] = useState<OrganizationWithRole | null>(null);
  const [webhooks, setWebhooks] = useState<Webhook[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // Create webhook modal
  const [showCreateModal, setShowCreateModal] = useState(false);
  const [newWebhook, setNewWebhook] = useState<CreateWebhookRequest>({
    name: "",
    url: "",
    events: [],
    description: "",
  });
  const [creating, setCreating] = useState(false);
  const [createdSecret, setCreatedSecret] = useState<string | null>(null);

  // Deliveries modal
  const [showDeliveriesModal, setShowDeliveriesModal] = useState(false);
  const [selectedWebhook, setSelectedWebhook] = useState<Webhook | null>(null);
  const [deliveries, setDeliveries] = useState<WebhookDelivery[]>([]);
  const [loadingDeliveries, setLoadingDeliveries] = useState(false);

  // Test webhook
  const [testingWebhookId, setTestingWebhookId] = useState<string | null>(null);

  const loadOrganizations = useCallback(async () => {
    try {
      const orgs = await api.listOrganizations();
      setOrganizations(orgs);
      if (orgs.length > 0 && !selectedOrg) {
        const nonPersonal = orgs.find((o) => !o.is_personal);
        setSelectedOrg(nonPersonal || orgs[0]);
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load organizations");
    }
  }, [selectedOrg]);

  const loadWebhooks = useCallback(async (orgId: string) => {
    try {
      setLoading(true);
      setError(null);
      const data = await api.listWebhooks(orgId);
      setWebhooks(data);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load webhooks");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    if (!authLoading && user) {
      loadOrganizations();
    }
  }, [authLoading, user, loadOrganizations]);

  useEffect(() => {
    if (selectedOrg) {
      loadWebhooks(selectedOrg.id);
    }
  }, [selectedOrg, loadWebhooks]);

  const handleCreateWebhook = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!selectedOrg || !newWebhook.name || !newWebhook.url || newWebhook.events.length === 0) {
      return;
    }

    try {
      setCreating(true);
      const created = await api.createWebhook(selectedOrg.id, newWebhook);
      setCreatedSecret(created.secret);
      setWebhooks([created, ...webhooks]);
      setNewWebhook({ name: "", url: "", events: [], description: "" });
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to create webhook");
    } finally {
      setCreating(false);
    }
  };

  const handleToggleActive = async (webhook: Webhook) => {
    if (!selectedOrg) return;
    try {
      const updated = await api.updateWebhook(selectedOrg.id, webhook.id, {
        is_active: !webhook.is_active,
      });
      setWebhooks(webhooks.map((w) => (w.id === webhook.id ? updated : w)));
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to update webhook");
    }
  };

  const handleDeleteWebhook = async (webhookId: string) => {
    if (!selectedOrg || !confirm("Are you sure you want to delete this webhook?")) return;
    try {
      await api.deleteWebhook(selectedOrg.id, webhookId);
      setWebhooks(webhooks.filter((w) => w.id !== webhookId));
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to delete webhook");
    }
  };

  const handleTestWebhook = async (webhookId: string) => {
    if (!selectedOrg) return;
    try {
      setTestingWebhookId(webhookId);
      const result = await api.sendTestWebhook(selectedOrg.id, webhookId);
      alert(`Test result: ${result.status} - ${result.message}`);
    } catch (err) {
      alert(`Test failed: ${err instanceof Error ? err.message : "Unknown error"}`);
    } finally {
      setTestingWebhookId(null);
    }
  };

  const handleViewDeliveries = async (webhook: Webhook) => {
    if (!selectedOrg) return;
    setSelectedWebhook(webhook);
    setShowDeliveriesModal(true);
    setLoadingDeliveries(true);
    try {
      const data = await api.listWebhookDeliveries(selectedOrg.id, webhook.id);
      setDeliveries(data);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load deliveries");
    } finally {
      setLoadingDeliveries(false);
    }
  };

  const toggleEvent = (event: string) => {
    if (newWebhook.events.includes(event)) {
      setNewWebhook({ ...newWebhook, events: newWebhook.events.filter((e) => e !== event) });
    } else {
      setNewWebhook({ ...newWebhook, events: [...newWebhook.events, event] });
    }
  };

  if (authLoading) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <div className="h-8 w-8 animate-spin rounded-full border-4 border-indigo-600 border-t-transparent" />
      </div>
    );
  }

  if (!user) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <div className="text-center">
          <h2 className="text-xl font-semibold text-gray-900 dark:text-white">Sign in required</h2>
          <p className="mt-2 text-gray-600 dark:text-gray-400">Please sign in to manage webhooks.</p>
          <a
            href={api.getLoginUrl()}
            className="mt-4 inline-block rounded-lg bg-indigo-600 px-4 py-2 text-white hover:bg-indigo-700"
          >
            Sign in with GitHub
          </a>
        </div>
      </div>
    );
  }

  return (
    <div className="p-6">
      {/* Header */}
      <div className="mb-6 flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-gray-900 dark:text-white">Webhooks</h1>
          <p className="text-sm text-gray-600 dark:text-gray-400">
            Receive notifications when events happen in your organization
          </p>
        </div>
        <button
          onClick={() => setShowCreateModal(true)}
          className="rounded-lg bg-indigo-600 px-4 py-2 text-sm font-medium text-white hover:bg-indigo-700"
        >
          Create Webhook
        </button>
      </div>

      {/* Organization Selector */}
      {organizations.length > 1 && (
        <div className="mb-6">
          <label className="block text-sm font-medium text-gray-700 dark:text-gray-300">Organization</label>
          <select
            value={selectedOrg?.id || ""}
            onChange={(e) => {
              const org = organizations.find((o) => o.id === e.target.value);
              setSelectedOrg(org || null);
            }}
            className="mt-1 block w-full max-w-xs rounded-lg border-gray-300 bg-white shadow-sm focus:border-indigo-500 focus:ring-indigo-500 dark:border-gray-600 dark:bg-gray-800 dark:text-white"
          >
            {organizations.map((org) => (
              <option key={org.id} value={org.id}>
                {org.name} {org.is_personal ? "(Personal)" : ""}
              </option>
            ))}
          </select>
        </div>
      )}

      {/* Error Alert */}
      {error && (
        <div className="mb-6 rounded-lg bg-red-50 p-4 text-red-700 dark:bg-red-900/20 dark:text-red-400">
          {error}
          <button onClick={() => setError(null)} className="ml-2 text-red-900 dark:text-red-300">
            Dismiss
          </button>
        </div>
      )}

      {/* Webhooks List */}
      {loading ? (
        <div className="flex justify-center py-12">
          <div className="h-8 w-8 animate-spin rounded-full border-4 border-indigo-600 border-t-transparent" />
        </div>
      ) : webhooks.length === 0 ? (
        <div className="rounded-lg border-2 border-dashed border-gray-300 p-12 text-center dark:border-gray-700">
          <svg
            className="mx-auto h-12 w-12 text-gray-400"
            fill="none"
            viewBox="0 0 24 24"
            strokeWidth={1.5}
            stroke="currentColor"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              d="M14.857 17.082a23.848 23.848 0 005.454-1.31A8.967 8.967 0 0118 9.75v-.7V9A6 6 0 006 9v.75a8.967 8.967 0 01-2.312 6.022c1.733.64 3.56 1.085 5.455 1.31m5.714 0a24.255 24.255 0 01-5.714 0m5.714 0a3 3 0 11-5.714 0"
            />
          </svg>
          <h3 className="mt-4 text-lg font-medium text-gray-900 dark:text-white">No webhooks</h3>
          <p className="mt-2 text-gray-600 dark:text-gray-400">
            Create a webhook to receive notifications about events.
          </p>
          <button
            onClick={() => setShowCreateModal(true)}
            className="mt-4 rounded-lg bg-indigo-600 px-4 py-2 text-sm font-medium text-white hover:bg-indigo-700"
          >
            Create your first webhook
          </button>
        </div>
      ) : (
        <div className="overflow-hidden rounded-lg border border-gray-200 dark:border-gray-700">
          <table className="min-w-full divide-y divide-gray-200 dark:divide-gray-700">
            <thead className="bg-gray-50 dark:bg-gray-800">
              <tr>
                <th className="px-6 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400">
                  Name
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400">
                  URL
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400">
                  Events
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400">
                  Status
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400">
                  Last Triggered
                </th>
                <th className="px-6 py-3 text-right text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400">
                  Actions
                </th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-200 bg-white dark:divide-gray-700 dark:bg-gray-900">
              {webhooks.map((webhook) => (
                <tr key={webhook.id}>
                  <td className="whitespace-nowrap px-6 py-4">
                    <div className="font-medium text-gray-900 dark:text-white">{webhook.name}</div>
                    {webhook.description && (
                      <div className="text-sm text-gray-500 dark:text-gray-400">{webhook.description}</div>
                    )}
                  </td>
                  <td className="max-w-xs truncate px-6 py-4 text-sm text-gray-500 dark:text-gray-400">
                    {webhook.url}
                  </td>
                  <td className="px-6 py-4">
                    <div className="flex flex-wrap gap-1">
                      {webhook.events.slice(0, 3).map((event) => (
                        <span
                          key={event}
                          className="inline-flex rounded-full bg-indigo-100 px-2 py-0.5 text-xs font-medium text-indigo-800 dark:bg-indigo-900/30 dark:text-indigo-400"
                        >
                          {event}
                        </span>
                      ))}
                      {webhook.events.length > 3 && (
                        <span className="text-xs text-gray-500 dark:text-gray-400">
                          +{webhook.events.length - 3} more
                        </span>
                      )}
                    </div>
                  </td>
                  <td className="whitespace-nowrap px-6 py-4">
                    <button
                      onClick={() => handleToggleActive(webhook)}
                      className={`inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium ${
                        webhook.is_active
                          ? "bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-400"
                          : "bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-400"
                      }`}
                    >
                      {webhook.is_active ? "Active" : "Inactive"}
                    </button>
                  </td>
                  <td className="whitespace-nowrap px-6 py-4 text-sm text-gray-500 dark:text-gray-400">
                    {webhook.last_triggered_at
                      ? new Date(webhook.last_triggered_at).toLocaleString()
                      : "Never"}
                  </td>
                  <td className="whitespace-nowrap px-6 py-4 text-right text-sm">
                    <div className="flex items-center justify-end gap-2">
                      <button
                        onClick={() => handleTestWebhook(webhook.id)}
                        disabled={testingWebhookId === webhook.id}
                        className="text-indigo-600 hover:text-indigo-900 dark:text-indigo-400 dark:hover:text-indigo-300 disabled:opacity-50"
                      >
                        {testingWebhookId === webhook.id ? "Testing..." : "Test"}
                      </button>
                      <button
                        onClick={() => handleViewDeliveries(webhook)}
                        className="text-indigo-600 hover:text-indigo-900 dark:text-indigo-400 dark:hover:text-indigo-300"
                      >
                        Deliveries
                      </button>
                      <button
                        onClick={() => handleDeleteWebhook(webhook.id)}
                        className="text-red-600 hover:text-red-900 dark:text-red-400 dark:hover:text-red-300"
                      >
                        Delete
                      </button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {/* Create Webhook Modal */}
      {showCreateModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
          <div className="w-full max-w-lg rounded-lg bg-white p-6 shadow-xl dark:bg-gray-800">
            <h2 className="text-lg font-semibold text-gray-900 dark:text-white">Create Webhook</h2>

            {createdSecret ? (
              <div className="mt-4">
                <div className="rounded-lg bg-green-50 p-4 dark:bg-green-900/20">
                  <h3 className="font-medium text-green-800 dark:text-green-400">Webhook created!</h3>
                  <p className="mt-2 text-sm text-green-700 dark:text-green-300">
                    Save this secret - it won&apos;t be shown again:
                  </p>
                  <code className="mt-2 block rounded bg-gray-100 p-2 text-sm dark:bg-gray-700">
                    {createdSecret}
                  </code>
                </div>
                <button
                  onClick={() => {
                    setShowCreateModal(false);
                    setCreatedSecret(null);
                  }}
                  className="mt-4 w-full rounded-lg bg-indigo-600 py-2 text-white hover:bg-indigo-700"
                >
                  Done
                </button>
              </div>
            ) : (
              <form onSubmit={handleCreateWebhook} className="mt-4 space-y-4">
                <div>
                  <label className="block text-sm font-medium text-gray-700 dark:text-gray-300">Name</label>
                  <input
                    type="text"
                    value={newWebhook.name}
                    onChange={(e) => setNewWebhook({ ...newWebhook, name: e.target.value })}
                    className="mt-1 block w-full rounded-lg border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 dark:border-gray-600 dark:bg-gray-700 dark:text-white"
                    placeholder="My Webhook"
                    required
                  />
                </div>

                <div>
                  <label className="block text-sm font-medium text-gray-700 dark:text-gray-300">
                    Payload URL
                  </label>
                  <input
                    type="url"
                    value={newWebhook.url}
                    onChange={(e) => setNewWebhook({ ...newWebhook, url: e.target.value })}
                    className="mt-1 block w-full rounded-lg border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 dark:border-gray-600 dark:bg-gray-700 dark:text-white"
                    placeholder="https://example.com/webhook"
                    required
                  />
                </div>

                <div>
                  <label className="block text-sm font-medium text-gray-700 dark:text-gray-300">
                    Description (optional)
                  </label>
                  <input
                    type="text"
                    value={newWebhook.description || ""}
                    onChange={(e) => setNewWebhook({ ...newWebhook, description: e.target.value })}
                    className="mt-1 block w-full rounded-lg border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 dark:border-gray-600 dark:bg-gray-700 dark:text-white"
                    placeholder="Slack notifications"
                  />
                </div>

                <div>
                  <label className="block text-sm font-medium text-gray-700 dark:text-gray-300">Events</label>
                  <div className="mt-2 grid grid-cols-2 gap-2">
                    {WEBHOOK_EVENT_TYPES.map((event) => (
                      <label
                        key={event}
                        className="flex items-center gap-2 rounded-lg border border-gray-200 p-2 dark:border-gray-600"
                      >
                        <input
                          type="checkbox"
                          checked={newWebhook.events.includes(event)}
                          onChange={() => toggleEvent(event)}
                          className="rounded border-gray-300 text-indigo-600 focus:ring-indigo-500"
                        />
                        <span className="text-sm text-gray-700 dark:text-gray-300">{event}</span>
                      </label>
                    ))}
                  </div>
                </div>

                <div className="flex gap-3 pt-4">
                  <button
                    type="button"
                    onClick={() => setShowCreateModal(false)}
                    className="flex-1 rounded-lg border border-gray-300 py-2 text-gray-700 hover:bg-gray-50 dark:border-gray-600 dark:text-gray-300 dark:hover:bg-gray-700"
                  >
                    Cancel
                  </button>
                  <button
                    type="submit"
                    disabled={creating || newWebhook.events.length === 0}
                    className="flex-1 rounded-lg bg-indigo-600 py-2 text-white hover:bg-indigo-700 disabled:opacity-50"
                  >
                    {creating ? "Creating..." : "Create Webhook"}
                  </button>
                </div>
              </form>
            )}
          </div>
        </div>
      )}

      {/* Deliveries Modal */}
      {showDeliveriesModal && selectedWebhook && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
          <div className="max-h-[80vh] w-full max-w-3xl overflow-hidden rounded-lg bg-white shadow-xl dark:bg-gray-800">
            <div className="flex items-center justify-between border-b border-gray-200 p-4 dark:border-gray-700">
              <h2 className="text-lg font-semibold text-gray-900 dark:text-white">
                Deliveries - {selectedWebhook.name}
              </h2>
              <button
                onClick={() => {
                  setShowDeliveriesModal(false);
                  setSelectedWebhook(null);
                  setDeliveries([]);
                }}
                className="text-gray-400 hover:text-gray-600 dark:hover:text-gray-300"
              >
                <svg className="h-6 w-6" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" d="M6 18L18 6M6 6l12 12" />
                </svg>
              </button>
            </div>

            <div className="max-h-[60vh] overflow-y-auto p-4">
              {loadingDeliveries ? (
                <div className="flex justify-center py-12">
                  <div className="h-8 w-8 animate-spin rounded-full border-4 border-indigo-600 border-t-transparent" />
                </div>
              ) : deliveries.length === 0 ? (
                <p className="py-12 text-center text-gray-500 dark:text-gray-400">No deliveries yet</p>
              ) : (
                <div className="space-y-3">
                  {deliveries.map((delivery) => (
                    <div
                      key={delivery.id}
                      className="rounded-lg border border-gray-200 p-4 dark:border-gray-700"
                    >
                      <div className="flex items-center justify-between">
                        <div className="flex items-center gap-3">
                          <span
                            className={`inline-flex rounded-full px-2 py-0.5 text-xs font-medium ${
                              delivery.status === "success"
                                ? "bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-400"
                                : delivery.status === "failed"
                                ? "bg-red-100 text-red-800 dark:bg-red-900/30 dark:text-red-400"
                                : delivery.status === "pending"
                                ? "bg-yellow-100 text-yellow-800 dark:bg-yellow-900/30 dark:text-yellow-400"
                                : "bg-blue-100 text-blue-800 dark:bg-blue-900/30 dark:text-blue-400"
                            }`}
                          >
                            {delivery.status}
                          </span>
                          <span className="text-sm font-medium text-gray-900 dark:text-white">
                            {delivery.event_type}
                          </span>
                        </div>
                        <div className="text-sm text-gray-500 dark:text-gray-400">
                          {new Date(delivery.created_at).toLocaleString()}
                        </div>
                      </div>
                      <div className="mt-2 flex items-center gap-4 text-sm text-gray-500 dark:text-gray-400">
                        {delivery.response_status && <span>Status: {delivery.response_status}</span>}
                        {delivery.duration_ms && <span>Duration: {delivery.duration_ms}ms</span>}
                        <span>Attempts: {delivery.attempt_count}</span>
                      </div>
                      {delivery.error_message && (
                        <div className="mt-2 rounded bg-red-50 p-2 text-sm text-red-700 dark:bg-red-900/20 dark:text-red-400">
                          {delivery.error_message}
                        </div>
                      )}
                    </div>
                  ))}
                </div>
              )}
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
