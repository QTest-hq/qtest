"use client";

import { useState, useEffect, useCallback } from "react";
import { useParams, useRouter } from "next/navigation";
import { useAuth } from "@/contexts/AuthContext";
import Sidebar from "@/components/Sidebar";
import {
  api,
  Webhook,
  WebhookDelivery,
  UpdateWebhookRequest,
  WEBHOOK_EVENT_TYPES,
} from "@/lib/api";

export default function WebhookDetailPage() {
  const params = useParams();
  const router = useRouter();
  const { user, loading: authLoading } = useAuth();
  const webhookId = params.id as string;

  const [webhook, setWebhook] = useState<Webhook | null>(null);
  const [deliveries, setDeliveries] = useState<WebhookDelivery[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // Edit mode state
  const [isEditing, setIsEditing] = useState(false);
  const [editForm, setEditForm] = useState<UpdateWebhookRequest>({});
  const [saving, setSaving] = useState(false);

  // Delete confirmation
  const [showDeleteConfirm, setShowDeleteConfirm] = useState(false);
  const [deleting, setDeleting] = useState(false);

  // Test webhook
  const [testing, setTesting] = useState(false);
  const [testResult, setTestResult] = useState<string | null>(null);

  // Regenerate secret
  const [regenerating, setRegenerating] = useState(false);
  const [newSecret, setNewSecret] = useState<string | null>(null);

  // Expanded delivery
  const [expandedDelivery, setExpandedDelivery] = useState<string | null>(null);

  const loadWebhook = useCallback(async () => {
    try {
      setLoading(true);
      // First get user orgs to find which org this webhook belongs to
      const orgs = await api.listOrganizations();

      // Try to find the webhook in each org
      for (const org of orgs) {
        try {
          const wh = await api.getWebhook(org.id, webhookId);
          if (wh) {
            setWebhook(wh);
            // Load deliveries
            const dels = await api.listWebhookDeliveries(org.id, webhookId);
            setDeliveries(dels);
            setError(null);
            return;
          }
        } catch {
          // Try next org
        }
      }
      setError("Webhook not found");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load webhook");
    } finally {
      setLoading(false);
    }
  }, [webhookId]);

  useEffect(() => {
    if (!authLoading && user) {
      loadWebhook();
    }
  }, [authLoading, user, loadWebhook]);

  const startEditing = () => {
    if (!webhook) return;
    setEditForm({
      name: webhook.name,
      url: webhook.url,
      events: [...webhook.events],
      description: webhook.description,
      is_active: webhook.is_active,
    });
    setIsEditing(true);
  };

  const cancelEditing = () => {
    setIsEditing(false);
    setEditForm({});
  };

  const handleSave = async () => {
    if (!webhook) return;
    try {
      setSaving(true);
      const updated = await api.updateWebhook(webhook.organization_id, webhook.id, editForm);
      setWebhook(updated);
      setIsEditing(false);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to update webhook");
    } finally {
      setSaving(false);
    }
  };

  const handleDelete = async () => {
    if (!webhook) return;
    try {
      setDeleting(true);
      await api.deleteWebhook(webhook.organization_id, webhook.id);
      router.push("/webhooks");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to delete webhook");
      setDeleting(false);
      setShowDeleteConfirm(false);
    }
  };

  const handleTest = async () => {
    if (!webhook) return;
    try {
      setTesting(true);
      setTestResult(null);
      const result = await api.sendTestWebhook(webhook.organization_id, webhook.id);
      setTestResult(`${result.status}: ${result.message}`);
      // Reload deliveries to show the test delivery
      const dels = await api.listWebhookDeliveries(webhook.organization_id, webhook.id);
      setDeliveries(dels);
    } catch (err) {
      setTestResult(`Error: ${err instanceof Error ? err.message : "Unknown error"}`);
    } finally {
      setTesting(false);
    }
  };

  const handleRegenerateSecret = async () => {
    if (!webhook || !confirm("Are you sure? The old secret will stop working immediately.")) return;
    try {
      setRegenerating(true);
      const result = await api.regenerateWebhookSecret(webhook.organization_id, webhook.id);
      setNewSecret(result.secret);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to regenerate secret");
    } finally {
      setRegenerating(false);
    }
  };

  const toggleEvent = (event: string) => {
    const events = editForm.events || [];
    if (events.includes(event)) {
      setEditForm({ ...editForm, events: events.filter((e) => e !== event) });
    } else {
      setEditForm({ ...editForm, events: [...events, event] });
    }
  };

  const getStatusBadge = (status: string) => {
    const colors: Record<string, string> = {
      success: "bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-400",
      failed: "bg-red-100 text-red-800 dark:bg-red-900/30 dark:text-red-400",
      pending: "bg-yellow-100 text-yellow-800 dark:bg-yellow-900/30 dark:text-yellow-400",
      retrying: "bg-blue-100 text-blue-800 dark:bg-blue-900/30 dark:text-blue-400",
    };
    return colors[status] || "bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-400";
  };

  if (authLoading || loading) {
    return (
      <div className="flex h-screen">
        <Sidebar />
        <main className="flex-1 flex items-center justify-center">
          <div className="h-8 w-8 animate-spin rounded-full border-4 border-indigo-600 border-t-transparent" />
          <span className="ml-3 text-gray-500">Loading webhook...</span>
        </main>
      </div>
    );
  }

  if (!user) {
    return (
      <div className="flex h-screen">
        <Sidebar />
        <main className="flex-1 flex items-center justify-center">
          <div className="text-center">
            <h2 className="text-xl font-semibold text-gray-900 dark:text-white">Sign in required</h2>
            <p className="mt-2 text-gray-600 dark:text-gray-400">Please sign in to view webhooks.</p>
          </div>
        </main>
      </div>
    );
  }

  if (error || !webhook) {
    return (
      <div className="flex h-screen">
        <Sidebar />
        <main className="flex-1 flex items-center justify-center">
          <div className="text-center">
            <svg className="mx-auto h-12 w-12 text-red-400" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" d="M12 9v3.75m9-.75a9 9 0 11-18 0 9 9 0 0118 0zm-9 3.75h.008v.008H12v-.008z" />
            </svg>
            <h3 className="mt-4 text-lg font-medium text-gray-900 dark:text-white">{error || "Webhook not found"}</h3>
            <a href="/webhooks" className="mt-4 inline-flex items-center text-indigo-600 hover:text-indigo-500">
              Back to webhooks
            </a>
          </div>
        </main>
      </div>
    );
  }

  return (
    <div className="flex h-screen">
      <Sidebar />
      <main className="flex-1 overflow-y-auto">
        {/* Header */}
        <div className="border-b border-gray-200 bg-white px-8 py-6 dark:border-gray-700 dark:bg-gray-800">
          <div className="flex items-center justify-between">
            <div className="flex items-center">
              <a href="/webhooks" className="mr-4 text-gray-400 hover:text-gray-500">
                <svg className="h-5 w-5" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" d="M10.5 19.5L3 12m0 0l7.5-7.5M3 12h18" />
                </svg>
              </a>
              <div>
                <h1 className="text-2xl font-semibold text-gray-900 dark:text-white">{webhook.name}</h1>
                <p className="text-sm text-gray-500 dark:text-gray-400">
                  {webhook.description || "No description"}
                </p>
              </div>
              <span className={`ml-4 inline-flex rounded-full px-3 py-1 text-sm font-semibold ${
                webhook.is_active
                  ? "bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-400"
                  : "bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-400"
              }`}>
                {webhook.is_active ? "Active" : "Inactive"}
              </span>
            </div>
            <div className="flex items-center space-x-3">
              {!isEditing && (
                <>
                  <button
                    onClick={handleTest}
                    disabled={testing}
                    className="inline-flex items-center rounded-lg border border-gray-300 bg-white px-4 py-2 text-sm font-medium text-gray-700 shadow-sm hover:bg-gray-50 disabled:opacity-50 dark:border-gray-600 dark:bg-gray-700 dark:text-gray-200 dark:hover:bg-gray-600"
                  >
                    {testing ? "Sending..." : "Send Test"}
                  </button>
                  <button
                    onClick={startEditing}
                    className="inline-flex items-center rounded-lg bg-indigo-600 px-4 py-2 text-sm font-medium text-white shadow-sm hover:bg-indigo-700"
                  >
                    Edit
                  </button>
                  <button
                    onClick={() => setShowDeleteConfirm(true)}
                    className="inline-flex items-center rounded-lg bg-red-600 px-4 py-2 text-sm font-medium text-white shadow-sm hover:bg-red-700"
                  >
                    Delete
                  </button>
                </>
              )}
            </div>
          </div>
        </div>

        <div className="p-8">
          {/* Test Result Banner */}
          {testResult && (
            <div className={`mb-6 rounded-lg p-4 ${
              testResult.startsWith("success")
                ? "bg-green-50 text-green-700 dark:bg-green-900/20 dark:text-green-400"
                : "bg-red-50 text-red-700 dark:bg-red-900/20 dark:text-red-400"
            }`}>
              {testResult}
              <button onClick={() => setTestResult(null)} className="ml-2 underline">Dismiss</button>
            </div>
          )}

          {/* New Secret Banner */}
          {newSecret && (
            <div className="mb-6 rounded-lg bg-green-50 p-4 dark:bg-green-900/20">
              <h3 className="font-medium text-green-800 dark:text-green-400">New secret generated!</h3>
              <p className="mt-1 text-sm text-green-700 dark:text-green-300">
                Save this secret - it won&apos;t be shown again:
              </p>
              <code className="mt-2 block rounded bg-gray-100 p-2 text-sm dark:bg-gray-700">{newSecret}</code>
              <button
                onClick={() => setNewSecret(null)}
                className="mt-2 text-green-800 underline dark:text-green-400"
              >
                Dismiss
              </button>
            </div>
          )}

          <div className="grid grid-cols-1 gap-6 lg:grid-cols-2">
            {/* Webhook Details */}
            <div className="rounded-lg bg-white p-6 shadow-sm ring-1 ring-gray-200 dark:bg-gray-800 dark:ring-gray-700">
              <h2 className="text-lg font-medium text-gray-900 dark:text-white mb-4">
                {isEditing ? "Edit Webhook" : "Webhook Details"}
              </h2>

              {isEditing ? (
                <form onSubmit={(e) => { e.preventDefault(); handleSave(); }} className="space-y-4">
                  <div>
                    <label className="block text-sm font-medium text-gray-700 dark:text-gray-300">Name</label>
                    <input
                      type="text"
                      value={editForm.name || ""}
                      onChange={(e) => setEditForm({ ...editForm, name: e.target.value })}
                      className="mt-1 block w-full rounded-lg border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 dark:border-gray-600 dark:bg-gray-700 dark:text-white"
                    />
                  </div>
                  <div>
                    <label className="block text-sm font-medium text-gray-700 dark:text-gray-300">URL</label>
                    <input
                      type="url"
                      value={editForm.url || ""}
                      onChange={(e) => setEditForm({ ...editForm, url: e.target.value })}
                      className="mt-1 block w-full rounded-lg border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 dark:border-gray-600 dark:bg-gray-700 dark:text-white"
                    />
                  </div>
                  <div>
                    <label className="block text-sm font-medium text-gray-700 dark:text-gray-300">Description</label>
                    <input
                      type="text"
                      value={editForm.description || ""}
                      onChange={(e) => setEditForm({ ...editForm, description: e.target.value })}
                      className="mt-1 block w-full rounded-lg border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 dark:border-gray-600 dark:bg-gray-700 dark:text-white"
                    />
                  </div>
                  <div>
                    <label className="flex items-center gap-2">
                      <input
                        type="checkbox"
                        checked={editForm.is_active ?? true}
                        onChange={(e) => setEditForm({ ...editForm, is_active: e.target.checked })}
                        className="rounded border-gray-300 text-indigo-600 focus:ring-indigo-500"
                      />
                      <span className="text-sm text-gray-700 dark:text-gray-300">Active</span>
                    </label>
                  </div>
                  <div>
                    <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">Events</label>
                    <div className="grid grid-cols-2 gap-2">
                      {WEBHOOK_EVENT_TYPES.map((event) => (
                        <label key={event} className="flex items-center gap-2">
                          <input
                            type="checkbox"
                            checked={editForm.events?.includes(event) ?? false}
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
                      onClick={cancelEditing}
                      className="flex-1 rounded-lg border border-gray-300 py-2 text-gray-700 hover:bg-gray-50 dark:border-gray-600 dark:text-gray-300 dark:hover:bg-gray-700"
                    >
                      Cancel
                    </button>
                    <button
                      type="submit"
                      disabled={saving}
                      className="flex-1 rounded-lg bg-indigo-600 py-2 text-white hover:bg-indigo-700 disabled:opacity-50"
                    >
                      {saving ? "Saving..." : "Save Changes"}
                    </button>
                  </div>
                </form>
              ) : (
                <dl className="space-y-3">
                  <div className="flex justify-between">
                    <dt className="text-sm text-gray-500 dark:text-gray-400">ID</dt>
                    <dd className="text-sm font-mono text-gray-900 dark:text-white">{webhook.id}</dd>
                  </div>
                  <div className="flex justify-between">
                    <dt className="text-sm text-gray-500 dark:text-gray-400">URL</dt>
                    <dd className="text-sm text-gray-900 dark:text-white max-w-xs truncate">{webhook.url}</dd>
                  </div>
                  <div>
                    <dt className="text-sm text-gray-500 dark:text-gray-400 mb-2">Events</dt>
                    <dd className="flex flex-wrap gap-1">
                      {webhook.events.map((event) => (
                        <span key={event} className="inline-flex rounded-full bg-indigo-100 px-2 py-0.5 text-xs font-medium text-indigo-800 dark:bg-indigo-900/30 dark:text-indigo-400">
                          {event}
                        </span>
                      ))}
                    </dd>
                  </div>
                  <div className="flex justify-between">
                    <dt className="text-sm text-gray-500 dark:text-gray-400">Created</dt>
                    <dd className="text-sm text-gray-900 dark:text-white">
                      {new Date(webhook.created_at).toLocaleString()}
                    </dd>
                  </div>
                  <div className="flex justify-between">
                    <dt className="text-sm text-gray-500 dark:text-gray-400">Last Triggered</dt>
                    <dd className="text-sm text-gray-900 dark:text-white">
                      {webhook.last_triggered_at ? new Date(webhook.last_triggered_at).toLocaleString() : "Never"}
                    </dd>
                  </div>
                </dl>
              )}
            </div>

            {/* Secret Management */}
            <div className="rounded-lg bg-white p-6 shadow-sm ring-1 ring-gray-200 dark:bg-gray-800 dark:ring-gray-700">
              <h2 className="text-lg font-medium text-gray-900 dark:text-white mb-4">Secret</h2>
              <p className="text-sm text-gray-600 dark:text-gray-400 mb-4">
                Use this secret to verify webhook payloads. The payload is signed with HMAC-SHA256.
              </p>
              <div className="flex items-center gap-2">
                <code className="flex-1 rounded bg-gray-100 p-2 text-sm dark:bg-gray-700">
                  whsec_**********************
                </code>
                <button
                  onClick={handleRegenerateSecret}
                  disabled={regenerating}
                  className="inline-flex items-center rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm font-medium text-gray-700 shadow-sm hover:bg-gray-50 disabled:opacity-50 dark:border-gray-600 dark:bg-gray-700 dark:text-gray-200 dark:hover:bg-gray-600"
                >
                  {regenerating ? "Regenerating..." : "Regenerate"}
                </button>
              </div>
            </div>
          </div>

          {/* Delivery History */}
          <div className="mt-8">
            <h2 className="text-lg font-medium text-gray-900 dark:text-white mb-4">
              Recent Deliveries ({deliveries.length})
            </h2>
            {deliveries.length === 0 ? (
              <div className="rounded-lg border-2 border-dashed border-gray-300 p-8 text-center dark:border-gray-700">
                <p className="text-gray-500 dark:text-gray-400">No deliveries yet</p>
              </div>
            ) : (
              <div className="space-y-3">
                {deliveries.map((delivery) => (
                  <div
                    key={delivery.id}
                    className="rounded-lg bg-white shadow-sm ring-1 ring-gray-200 dark:bg-gray-800 dark:ring-gray-700 overflow-hidden"
                  >
                    <div
                      className="flex items-center justify-between p-4 cursor-pointer hover:bg-gray-50 dark:hover:bg-gray-700"
                      onClick={() => setExpandedDelivery(expandedDelivery === delivery.id ? null : delivery.id)}
                    >
                      <div className="flex items-center gap-3">
                        <span className={`inline-flex rounded-full px-2 py-0.5 text-xs font-medium ${getStatusBadge(delivery.status)}`}>
                          {delivery.status}
                        </span>
                        <span className="text-sm font-medium text-gray-900 dark:text-white">{delivery.event_type}</span>
                        {delivery.response_status && (
                          <span className="text-sm text-gray-500 dark:text-gray-400">HTTP {delivery.response_status}</span>
                        )}
                      </div>
                      <div className="flex items-center gap-4">
                        {delivery.duration_ms && (
                          <span className="text-sm text-gray-500 dark:text-gray-400">{delivery.duration_ms}ms</span>
                        )}
                        <span className="text-sm text-gray-500 dark:text-gray-400">
                          {new Date(delivery.created_at).toLocaleString()}
                        </span>
                        <svg className={`h-5 w-5 text-gray-400 transition-transform ${expandedDelivery === delivery.id ? "rotate-180" : ""}`} fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor">
                          <path strokeLinecap="round" strokeLinejoin="round" d="M19.5 8.25l-7.5 7.5-7.5-7.5" />
                        </svg>
                      </div>
                    </div>

                    {expandedDelivery === delivery.id && (
                      <div className="border-t border-gray-200 p-4 dark:border-gray-700">
                        <div className="grid grid-cols-2 gap-4 text-sm">
                          <div>
                            <dt className="font-medium text-gray-500 dark:text-gray-400">Delivery ID</dt>
                            <dd className="font-mono text-gray-900 dark:text-white">{delivery.id}</dd>
                          </div>
                          <div>
                            <dt className="font-medium text-gray-500 dark:text-gray-400">Attempts</dt>
                            <dd className="text-gray-900 dark:text-white">{delivery.attempt_count}</dd>
                          </div>
                          {delivery.error_message && (
                            <div className="col-span-2">
                              <dt className="font-medium text-gray-500 dark:text-gray-400">Error</dt>
                              <dd className="mt-1 rounded bg-red-50 p-2 text-red-700 dark:bg-red-900/20 dark:text-red-400">
                                {delivery.error_message}
                              </dd>
                            </div>
                          )}
                        </div>
                      </div>
                    )}
                  </div>
                ))}
              </div>
            )}
          </div>
        </div>

        {/* Delete Confirmation Modal */}
        {showDeleteConfirm && (
          <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
            <div className="w-full max-w-md rounded-lg bg-white p-6 shadow-xl dark:bg-gray-800">
              <h2 className="text-lg font-semibold text-gray-900 dark:text-white">Delete Webhook</h2>
              <p className="mt-2 text-gray-600 dark:text-gray-400">
                Are you sure you want to delete &quot;{webhook.name}&quot;? This action cannot be undone.
              </p>
              <div className="mt-4 flex gap-3">
                <button
                  onClick={() => setShowDeleteConfirm(false)}
                  className="flex-1 rounded-lg border border-gray-300 py-2 text-gray-700 hover:bg-gray-50 dark:border-gray-600 dark:text-gray-300 dark:hover:bg-gray-700"
                >
                  Cancel
                </button>
                <button
                  onClick={handleDelete}
                  disabled={deleting}
                  className="flex-1 rounded-lg bg-red-600 py-2 text-white hover:bg-red-700 disabled:opacity-50"
                >
                  {deleting ? "Deleting..." : "Delete"}
                </button>
              </div>
            </div>
          </div>
        )}
      </main>
    </div>
  );
}
