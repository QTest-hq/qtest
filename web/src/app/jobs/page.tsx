"use client";

import { useEffect, useState } from "react";
import Sidebar from "@/components/Sidebar";
import { api, Job } from "@/lib/api";

export default function JobsPage() {
  const [jobs, setJobs] = useState<Job[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [filter, setFilter] = useState<string>("all");

  useEffect(() => {
    loadJobs();
  }, [filter]);

  async function loadJobs() {
    try {
      setLoading(true);
      const params: { status?: string; limit?: number } = { limit: 50 };
      if (filter !== "all") {
        params.status = filter;
      }
      const data = await api.listJobs(params);
      setJobs(data);
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load jobs");
    } finally {
      setLoading(false);
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

  function getJobTypeLabel(type: string): string {
    const labels: Record<string, string> = {
      pipeline: "Pipeline",
      clone: "Clone",
      analyze: "Analyze",
      generate: "Generate",
      validate: "Validate",
      mutation: "Mutation",
    };
    return labels[type] || type;
  }

  return (
    <div className="flex h-screen">
      <Sidebar />

      <main className="flex-1 overflow-y-auto">
        {/* Header */}
        <div className="border-b border-gray-200 bg-white px-8 py-6 dark:border-gray-700 dark:bg-gray-800">
          <div className="flex items-center justify-between">
            <div>
              <h1 className="text-2xl font-semibold text-gray-900 dark:text-white">
                Jobs
              </h1>
              <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
                Monitor test generation pipelines
              </p>
            </div>
            <a
              href="/jobs/new"
              className="inline-flex items-center rounded-lg bg-indigo-600 px-4 py-2 text-sm font-medium text-white shadow-sm hover:bg-indigo-700"
            >
              <svg
                className="mr-2 h-4 w-4"
                fill="none"
                viewBox="0 0 24 24"
                strokeWidth={1.5}
                stroke="currentColor"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  d="M5.25 5.653c0-.856.917-1.398 1.667-.986l11.54 6.347a1.125 1.125 0 010 1.972l-11.54 6.347a1.125 1.125 0 01-1.667-.986V5.653z"
                />
              </svg>
              Start Pipeline
            </a>
          </div>
        </div>

        <div className="p-8">
          {/* Filters */}
          <div className="mb-6 flex items-center space-x-2">
            <span className="text-sm text-gray-500 dark:text-gray-400">Filter:</span>
            {["all", "pending", "running", "completed", "failed"].map((status) => (
              <button
                key={status}
                onClick={() => setFilter(status)}
                className={`rounded-lg px-3 py-1 text-sm font-medium transition-colors ${
                  filter === status
                    ? "bg-indigo-600 text-white"
                    : "bg-gray-100 text-gray-700 hover:bg-gray-200 dark:bg-gray-700 dark:text-gray-300 dark:hover:bg-gray-600"
                }`}
              >
                {status.charAt(0).toUpperCase() + status.slice(1)}
              </button>
            ))}
          </div>

          {loading ? (
            <div className="flex items-center justify-center py-12">
              <div className="h-8 w-8 animate-spin rounded-full border-4 border-indigo-600 border-t-transparent" />
              <span className="ml-3 text-gray-500">Loading jobs...</span>
            </div>
          ) : error ? (
            <div className="rounded-lg bg-red-50 p-4 dark:bg-red-900/20">
              <div className="flex">
                <svg
                  className="h-5 w-5 text-red-400"
                  viewBox="0 0 20 20"
                  fill="currentColor"
                >
                  <path
                    fillRule="evenodd"
                    d="M10 18a8 8 0 100-16 8 8 0 000 16zM8.28 7.22a.75.75 0 00-1.06 1.06L8.94 10l-1.72 1.72a.75.75 0 101.06 1.06L10 11.06l1.72 1.72a.75.75 0 101.06-1.06L11.06 10l1.72-1.72a.75.75 0 00-1.06-1.06L10 8.94 8.28 7.22z"
                    clipRule="evenodd"
                  />
                </svg>
                <div className="ml-3">
                  <p className="text-sm font-medium text-red-800 dark:text-red-200">
                    {error}
                  </p>
                  <button
                    onClick={loadJobs}
                    className="mt-2 text-sm text-red-600 hover:text-red-500 dark:text-red-400"
                  >
                    Try again
                  </button>
                </div>
              </div>
            </div>
          ) : jobs.length === 0 ? (
            <div className="text-center py-12">
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
                  d="M3.75 12h16.5m-16.5 3.75h16.5M3.75 19.5h16.5M5.625 4.5h12.75a1.875 1.875 0 010 3.75H5.625a1.875 1.875 0 010-3.75z"
                />
              </svg>
              <h3 className="mt-4 text-lg font-medium text-gray-900 dark:text-white">
                No jobs found
              </h3>
              <p className="mt-2 text-sm text-gray-500 dark:text-gray-400">
                {filter !== "all"
                  ? `No ${filter} jobs at the moment.`
                  : "Start a pipeline to generate tests."}
              </p>
              <a
                href="/jobs/new"
                className="mt-4 inline-flex items-center rounded-lg bg-indigo-600 px-4 py-2 text-sm font-medium text-white shadow-sm hover:bg-indigo-700"
              >
                Start Pipeline
              </a>
            </div>
          ) : (
            <div className="space-y-4">
              {jobs.map((job) => (
                <div
                  key={job.id}
                  className="rounded-lg bg-white p-6 shadow-sm ring-1 ring-gray-200 dark:bg-gray-800 dark:ring-gray-700"
                >
                  <div className="flex items-center justify-between">
                    <div className="flex items-center">
                      <span
                        className={`inline-flex rounded-full px-2 py-1 text-xs font-semibold ${getStatusColor(
                          job.status
                        )}`}
                      >
                        {job.status}
                      </span>
                      <span className="ml-3 text-sm font-medium text-gray-900 dark:text-white">
                        {getJobTypeLabel(job.type)}
                      </span>
                      <span className="ml-2 text-xs text-gray-500 dark:text-gray-400">
                        #{job.id.slice(0, 8)}
                      </span>
                    </div>
                    <div className="flex items-center space-x-2">
                      {job.status === "running" && (
                        <button
                          onClick={() => api.cancelJob(job.id).then(loadJobs)}
                          className="text-sm text-red-600 hover:text-red-700 dark:text-red-400"
                        >
                          Cancel
                        </button>
                      )}
                      {job.status === "failed" && (
                        <button
                          onClick={() => api.retryJob(job.id).then(loadJobs)}
                          className="text-sm text-indigo-600 hover:text-indigo-700 dark:text-indigo-400"
                        >
                          Retry
                        </button>
                      )}
                    </div>
                  </div>

                  {job.error_message && (
                    <div className="mt-3 rounded bg-red-50 px-3 py-2 text-sm text-red-700 dark:bg-red-900/20 dark:text-red-300">
                      {job.error_message}
                    </div>
                  )}

                  <div className="mt-4 flex items-center text-xs text-gray-500 dark:text-gray-400">
                    <span>Created: {new Date(job.created_at).toLocaleString()}</span>
                    {job.started_at && (
                      <span className="ml-4">
                        Started: {new Date(job.started_at).toLocaleString()}
                      </span>
                    )}
                    {job.completed_at && (
                      <span className="ml-4">
                        Completed: {new Date(job.completed_at).toLocaleString()}
                      </span>
                    )}
                    <span className="ml-4">
                      Retries: {job.retry_count}/{job.max_retries}
                    </span>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      </main>
    </div>
  );
}
