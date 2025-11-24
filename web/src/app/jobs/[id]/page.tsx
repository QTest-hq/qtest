"use client";

import { useEffect, useState, useCallback } from "react";
import { useParams, useRouter } from "next/navigation";
import Sidebar from "@/components/Sidebar";
import { api, Job, GeneratedTest } from "@/lib/api";

const POLL_INTERVAL = 3000; // 3 seconds for running jobs

export default function JobDetailPage() {
  const params = useParams();
  const router = useRouter();
  const id = params.id as string;

  const [job, setJob] = useState<Job | null>(null);
  const [childJobs, setChildJobs] = useState<Job[]>([]);
  const [tests, setTests] = useState<GeneratedTest[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [actionLoading, setActionLoading] = useState<string | null>(null);

  const loadData = useCallback(async () => {
    try {
      const [jobData, childrenData] = await Promise.all([
        api.getJob(id),
        api.getJobChildren(id).catch(() => []),
      ]);
      setJob(jobData);
      setChildJobs(childrenData);

      // Load related tests if this is a generate job
      if (jobData.generation_run_id) {
        const testsData = await api.listTests({ run_id: jobData.generation_run_id, limit: 50 }).catch(() => []);
        setTests(testsData);
      }

      setError(null);
      return jobData;
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load job");
      return null;
    }
  }, [id]);

  useEffect(() => {
    async function initialLoad() {
      setLoading(true);
      await loadData();
      setLoading(false);
    }
    initialLoad();
  }, [loadData]);

  // Poll for updates when job is running or pending
  useEffect(() => {
    if (!job || (job.status !== "running" && job.status !== "pending")) {
      return;
    }

    const interval = setInterval(async () => {
      const updatedJob = await loadData();
      // Stop polling if job is no longer active
      if (updatedJob && updatedJob.status !== "running" && updatedJob.status !== "pending") {
        clearInterval(interval);
      }
    }, POLL_INTERVAL);

    return () => clearInterval(interval);
  }, [job?.status, loadData]);

  async function handleCancel() {
    try {
      setActionLoading("cancel");
      await api.cancelJob(id);
      await loadData();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to cancel job");
    } finally {
      setActionLoading(null);
    }
  }

  async function handleRetry() {
    try {
      setActionLoading("retry");
      const newJob = await api.retryJob(id);
      router.push(`/jobs/${newJob.id}`);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to retry job");
      setActionLoading(null);
    }
  }

  function getStatusColor(status: string): string {
    switch (status) {
      case "completed":
        return "bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200";
      case "running":
        return "bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200";
      case "pending":
        return "bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-200";
      case "failed":
        return "bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200";
      case "cancelled":
        return "bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-200";
      default:
        return "bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-200";
    }
  }

  function getStatusIcon(status: string) {
    switch (status) {
      case "completed":
        return (
          <svg className="h-5 w-5 text-green-500" fill="none" viewBox="0 0 24 24" strokeWidth={2} stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" d="M9 12.75L11.25 15 15 9.75M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
          </svg>
        );
      case "running":
        return (
          <div className="h-5 w-5 animate-spin rounded-full border-2 border-blue-500 border-t-transparent" />
        );
      case "pending":
        return (
          <svg className="h-5 w-5 text-yellow-500" fill="none" viewBox="0 0 24 24" strokeWidth={2} stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" d="M12 6v6h4.5m4.5 0a9 9 0 11-18 0 9 9 0 0118 0z" />
          </svg>
        );
      case "failed":
        return (
          <svg className="h-5 w-5 text-red-500" fill="none" viewBox="0 0 24 24" strokeWidth={2} stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" d="M9.75 9.75l4.5 4.5m0-4.5l-4.5 4.5M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
          </svg>
        );
      default:
        return (
          <svg className="h-5 w-5 text-gray-400" fill="none" viewBox="0 0 24 24" strokeWidth={2} stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" d="M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
          </svg>
        );
    }
  }

  function getJobTypeLabel(type: string): string {
    const labels: Record<string, string> = {
      pipeline: "Pipeline",
      clone: "Clone Repository",
      analyze: "Analyze Code",
      generate: "Generate Tests",
      validate: "Validate Tests",
      mutation: "Mutation Testing",
    };
    return labels[type] || type;
  }

  function calculateDuration(start?: string, end?: string): string {
    if (!start) return "-";
    const startDate = new Date(start);
    const endDate = end ? new Date(end) : new Date();
    const diff = endDate.getTime() - startDate.getTime();
    const seconds = Math.floor(diff / 1000);
    const minutes = Math.floor(seconds / 60);
    const hours = Math.floor(minutes / 60);

    if (hours > 0) return `${hours}h ${minutes % 60}m`;
    if (minutes > 0) return `${minutes}m ${seconds % 60}s`;
    return `${seconds}s`;
  }

  if (loading) {
    return (
      <div className="flex h-screen">
        <Sidebar />
        <main className="flex-1 flex items-center justify-center">
          <div className="h-8 w-8 animate-spin rounded-full border-4 border-indigo-600 border-t-transparent" />
          <span className="ml-3 text-gray-500">Loading job...</span>
        </main>
      </div>
    );
  }

  if (error || !job) {
    return (
      <div className="flex h-screen">
        <Sidebar />
        <main className="flex-1 flex items-center justify-center">
          <div className="text-center">
            <svg className="mx-auto h-12 w-12 text-red-400" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" d="M12 9v3.75m9-.75a9 9 0 11-18 0 9 9 0 0118 0zm-9 3.75h.008v.008H12v-.008z" />
            </svg>
            <h3 className="mt-4 text-lg font-medium text-gray-900 dark:text-white">
              {error || "Job not found"}
            </h3>
            <a href="/jobs" className="mt-4 inline-flex items-center text-indigo-600 hover:text-indigo-500">
              Back to jobs
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
              <a href="/jobs" className="mr-4 text-gray-400 hover:text-gray-500">
                <svg className="h-5 w-5" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" d="M10.5 19.5L3 12m0 0l7.5-7.5M3 12h18" />
                </svg>
              </a>
              <div className="flex items-center">
                {getStatusIcon(job.status)}
                <div className="ml-3">
                  <h1 className="text-2xl font-semibold text-gray-900 dark:text-white">
                    {getJobTypeLabel(job.type)}
                  </h1>
                  <p className="text-sm text-gray-500 dark:text-gray-400">
                    #{job.id.slice(0, 8)}
                  </p>
                </div>
                <span className={`ml-4 inline-flex rounded-full px-3 py-1 text-sm font-semibold ${getStatusColor(job.status)}`}>
                  {job.status}
                </span>
                {(job.status === "running" || job.status === "pending") && (
                  <span className="ml-2 text-xs text-gray-500 animate-pulse">
                    Auto-refreshing...
                  </span>
                )}
              </div>
            </div>
            <div className="flex items-center space-x-3">
              {job.status === "running" && (
                <button
                  onClick={handleCancel}
                  disabled={actionLoading === "cancel"}
                  className="inline-flex items-center rounded-lg bg-red-600 px-4 py-2 text-sm font-medium text-white shadow-sm hover:bg-red-700 disabled:opacity-50"
                >
                  {actionLoading === "cancel" ? "Cancelling..." : "Cancel"}
                </button>
              )}
              {job.status === "failed" && (
                <button
                  onClick={handleRetry}
                  disabled={actionLoading === "retry"}
                  className="inline-flex items-center rounded-lg bg-indigo-600 px-4 py-2 text-sm font-medium text-white shadow-sm hover:bg-indigo-700 disabled:opacity-50"
                >
                  {actionLoading === "retry" ? "Retrying..." : "Retry"}
                </button>
              )}
            </div>
          </div>
        </div>

        <div className="p-8">
          {/* Progress indicator for running jobs */}
          {job.status === "running" && (
            <div className="mb-6 rounded-lg bg-blue-50 p-4 dark:bg-blue-900/20">
              <div className="flex items-center">
                <div className="h-4 w-4 animate-spin rounded-full border-2 border-blue-500 border-t-transparent mr-3" />
                <span className="text-blue-700 dark:text-blue-300">Job is currently running...</span>
              </div>
              <div className="mt-3 h-2 rounded-full bg-blue-200 dark:bg-blue-800 overflow-hidden">
                <div className="h-full bg-blue-500 animate-pulse" style={{ width: "60%" }} />
              </div>
            </div>
          )}

          {/* Error message */}
          {job.error_message && (
            <div className="mb-6 rounded-lg bg-red-50 p-4 dark:bg-red-900/20">
              <div className="flex">
                <svg className="h-5 w-5 text-red-400" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" d="M12 9v3.75m9-.75a9 9 0 11-18 0 9 9 0 0118 0zm-9 3.75h.008v.008H12v-.008z" />
                </svg>
                <div className="ml-3">
                  <h3 className="text-sm font-medium text-red-800 dark:text-red-200">Error</h3>
                  <p className="mt-1 text-sm text-red-700 dark:text-red-300">{job.error_message}</p>
                </div>
              </div>
            </div>
          )}

          {/* Job Details */}
          <div className="grid grid-cols-1 gap-6 lg:grid-cols-2">
            <div className="rounded-lg bg-white p-6 shadow-sm ring-1 ring-gray-200 dark:bg-gray-800 dark:ring-gray-700">
              <h2 className="text-lg font-medium text-gray-900 dark:text-white mb-4">Job Details</h2>
              <dl className="space-y-3">
                <div className="flex justify-between">
                  <dt className="text-sm text-gray-500 dark:text-gray-400">Job ID</dt>
                  <dd className="text-sm font-mono text-gray-900 dark:text-white">{job.id}</dd>
                </div>
                <div className="flex justify-between">
                  <dt className="text-sm text-gray-500 dark:text-gray-400">Type</dt>
                  <dd className="text-sm font-medium text-gray-900 dark:text-white">{getJobTypeLabel(job.type)}</dd>
                </div>
                <div className="flex justify-between">
                  <dt className="text-sm text-gray-500 dark:text-gray-400">Priority</dt>
                  <dd className="text-sm text-gray-900 dark:text-white">{job.priority}</dd>
                </div>
                <div className="flex justify-between">
                  <dt className="text-sm text-gray-500 dark:text-gray-400">Retries</dt>
                  <dd className="text-sm text-gray-900 dark:text-white">{job.retry_count} / {job.max_retries}</dd>
                </div>
                {job.repository_id && (
                  <div className="flex justify-between">
                    <dt className="text-sm text-gray-500 dark:text-gray-400">Repository</dt>
                    <dd>
                      <a href={`/repos/${job.repository_id}`} className="text-sm text-indigo-600 hover:text-indigo-500">
                        View
                      </a>
                    </dd>
                  </div>
                )}
                {job.parent_job_id && (
                  <div className="flex justify-between">
                    <dt className="text-sm text-gray-500 dark:text-gray-400">Parent Job</dt>
                    <dd>
                      <a href={`/jobs/${job.parent_job_id}`} className="text-sm text-indigo-600 hover:text-indigo-500">
                        #{job.parent_job_id.slice(0, 8)}
                      </a>
                    </dd>
                  </div>
                )}
              </dl>
            </div>

            <div className="rounded-lg bg-white p-6 shadow-sm ring-1 ring-gray-200 dark:bg-gray-800 dark:ring-gray-700">
              <h2 className="text-lg font-medium text-gray-900 dark:text-white mb-4">Timing</h2>
              <dl className="space-y-3">
                <div className="flex justify-between">
                  <dt className="text-sm text-gray-500 dark:text-gray-400">Created</dt>
                  <dd className="text-sm text-gray-900 dark:text-white">{new Date(job.created_at).toLocaleString()}</dd>
                </div>
                {job.started_at && (
                  <div className="flex justify-between">
                    <dt className="text-sm text-gray-500 dark:text-gray-400">Started</dt>
                    <dd className="text-sm text-gray-900 dark:text-white">{new Date(job.started_at).toLocaleString()}</dd>
                  </div>
                )}
                {job.completed_at && (
                  <div className="flex justify-between">
                    <dt className="text-sm text-gray-500 dark:text-gray-400">Completed</dt>
                    <dd className="text-sm text-gray-900 dark:text-white">{new Date(job.completed_at).toLocaleString()}</dd>
                  </div>
                )}
                <div className="flex justify-between">
                  <dt className="text-sm text-gray-500 dark:text-gray-400">Duration</dt>
                  <dd className="text-sm font-medium text-gray-900 dark:text-white">
                    {calculateDuration(job.started_at, job.completed_at)}
                  </dd>
                </div>
              </dl>
            </div>
          </div>

          {/* Child Jobs */}
          {childJobs.length > 0 && (
            <div className="mt-8">
              <h2 className="text-lg font-medium text-gray-900 dark:text-white mb-4">Sub-Jobs</h2>
              <div className="space-y-3">
                {childJobs.map((child) => (
                  <a
                    key={child.id}
                    href={`/jobs/${child.id}`}
                    className="block rounded-lg bg-white p-4 shadow-sm ring-1 ring-gray-200 hover:ring-indigo-500 dark:bg-gray-800 dark:ring-gray-700 dark:hover:ring-indigo-400 transition-all"
                  >
                    <div className="flex items-center justify-between">
                      <div className="flex items-center">
                        {getStatusIcon(child.status)}
                        <span className="ml-3 text-sm font-medium text-gray-900 dark:text-white">
                          {getJobTypeLabel(child.type)}
                        </span>
                        <span className="ml-2 text-xs text-gray-500 dark:text-gray-400">
                          #{child.id.slice(0, 8)}
                        </span>
                      </div>
                      <div className="flex items-center">
                        <span className={`inline-flex rounded-full px-2 py-1 text-xs font-semibold ${getStatusColor(child.status)}`}>
                          {child.status}
                        </span>
                        <span className="ml-4 text-xs text-gray-500 dark:text-gray-400">
                          {calculateDuration(child.started_at, child.completed_at)}
                        </span>
                      </div>
                    </div>
                    {child.error_message && (
                      <p className="mt-2 text-sm text-red-600 dark:text-red-400 truncate">
                        {child.error_message}
                      </p>
                    )}
                  </a>
                ))}
              </div>
            </div>
          )}

          {/* Generated Tests */}
          {tests.length > 0 && (
            <div className="mt-8">
              <h2 className="text-lg font-medium text-gray-900 dark:text-white mb-4">
                Generated Tests ({tests.length})
              </h2>
              <div className="overflow-hidden rounded-lg bg-white shadow-sm ring-1 ring-gray-200 dark:bg-gray-800 dark:ring-gray-700">
                <table className="min-w-full divide-y divide-gray-200 dark:divide-gray-700">
                  <thead className="bg-gray-50 dark:bg-gray-900">
                    <tr>
                      <th className="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400">
                        Test Name
                      </th>
                      <th className="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400">
                        Target Function
                      </th>
                      <th className="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400">
                        Status
                      </th>
                      <th className="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400">
                        Mutation Score
                      </th>
                      <th className="px-4 py-3 text-right text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400">
                        Actions
                      </th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-gray-200 dark:divide-gray-700">
                    {tests.map((test) => (
                      <tr key={test.id} className="hover:bg-gray-50 dark:hover:bg-gray-700">
                        <td className="px-4 py-3 whitespace-nowrap">
                          <a href={`/tests/${test.id}`} className="text-sm font-medium text-indigo-600 hover:text-indigo-500 dark:text-indigo-400">
                            {test.name}
                          </a>
                        </td>
                        <td className="px-4 py-3 whitespace-nowrap text-sm text-gray-500 dark:text-gray-400">
                          {test.target_function || "-"}
                        </td>
                        <td className="px-4 py-3 whitespace-nowrap">
                          <span className={`inline-flex rounded-full px-2 py-1 text-xs font-semibold ${getStatusColor(test.status)}`}>
                            {test.status}
                          </span>
                        </td>
                        <td className="px-4 py-3 whitespace-nowrap text-sm text-gray-900 dark:text-white">
                          {test.mutation_score !== undefined ? `${(test.mutation_score * 100).toFixed(0)}%` : "-"}
                        </td>
                        <td className="px-4 py-3 whitespace-nowrap text-right text-sm">
                          {test.status === "pending" && (
                            <>
                              <button
                                onClick={() => api.acceptTest(test.id).then(loadData)}
                                className="text-green-600 hover:text-green-700 mr-3"
                              >
                                Accept
                              </button>
                              <button
                                onClick={() => api.rejectTest(test.id).then(loadData)}
                                className="text-red-600 hover:text-red-700"
                              >
                                Reject
                              </button>
                            </>
                          )}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </div>
          )}
        </div>
      </main>
    </div>
  );
}
