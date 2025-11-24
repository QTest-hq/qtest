"use client";

import { useEffect, useState } from "react";
import { useParams, useRouter } from "next/navigation";
import Sidebar from "@/components/Sidebar";
import { api, Repository, Job, GeneratedTest } from "@/lib/api";

export default function RepositoryDetailPage() {
  const params = useParams();
  const router = useRouter();
  const id = params.id as string;

  const [repo, setRepo] = useState<Repository | null>(null);
  const [jobs, setJobs] = useState<Job[]>([]);
  const [tests, setTests] = useState<GeneratedTest[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [deleting, setDeleting] = useState(false);

  useEffect(() => {
    loadData();
  }, [id]);

  async function loadData() {
    try {
      setLoading(true);
      const [repoData, jobsData, testsData] = await Promise.all([
        api.getRepo(id),
        api.getJobsByRepo(id, 10).catch(() => []),
        api.listTests({ limit: 20 }).catch(() => []),
      ]);
      setRepo(repoData);
      setJobs(jobsData);
      // Filter tests that might be related to this repo (if they have repo context)
      setTests(testsData.slice(0, 10));
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load repository");
    } finally {
      setLoading(false);
    }
  }

  async function handleDelete() {
    if (!confirm("Are you sure you want to delete this repository? This action cannot be undone.")) {
      return;
    }
    try {
      setDeleting(true);
      await api.deleteRepo(id);
      router.push("/repos");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to delete repository");
      setDeleting(false);
    }
  }

  function getStatusColor(status: string): string {
    switch (status) {
      case "ready":
        return "bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200";
      case "cloning":
      case "analyzing":
        return "bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-200";
      case "error":
        return "bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200";
      default:
        return "bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-200";
    }
  }

  function getJobStatusColor(status: string): string {
    switch (status) {
      case "completed":
        return "bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200";
      case "running":
        return "bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200";
      case "pending":
        return "bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-200";
      case "failed":
        return "bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200";
      default:
        return "bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-200";
    }
  }

  if (loading) {
    return (
      <div className="flex h-screen">
        <Sidebar />
        <main className="flex-1 flex items-center justify-center">
          <div className="h-8 w-8 animate-spin rounded-full border-4 border-indigo-600 border-t-transparent" />
          <span className="ml-3 text-gray-500">Loading repository...</span>
        </main>
      </div>
    );
  }

  if (error || !repo) {
    return (
      <div className="flex h-screen">
        <Sidebar />
        <main className="flex-1 flex items-center justify-center">
          <div className="text-center">
            <svg className="mx-auto h-12 w-12 text-red-400" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" d="M12 9v3.75m9-.75a9 9 0 11-18 0 9 9 0 0118 0zm-9 3.75h.008v.008H12v-.008z" />
            </svg>
            <h3 className="mt-4 text-lg font-medium text-gray-900 dark:text-white">
              {error || "Repository not found"}
            </h3>
            <a href="/repos" className="mt-4 inline-flex items-center text-indigo-600 hover:text-indigo-500">
              Back to repositories
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
              <a href="/repos" className="mr-4 text-gray-400 hover:text-gray-500">
                <svg className="h-5 w-5" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" d="M10.5 19.5L3 12m0 0l7.5-7.5M3 12h18" />
                </svg>
              </a>
              <div>
                <div className="flex items-center">
                  <svg className="h-6 w-6 text-gray-400 mr-2" fill="currentColor" viewBox="0 0 24 24">
                    <path fillRule="evenodd" d="M12 2C6.477 2 2 6.484 2 12.017c0 4.425 2.865 8.18 6.839 9.504.5.092.682-.217.682-.483 0-.237-.008-.868-.013-1.703-2.782.605-3.369-1.343-3.369-1.343-.454-1.158-1.11-1.466-1.11-1.466-.908-.62.069-.608.069-.608 1.003.07 1.531 1.032 1.531 1.032.892 1.53 2.341 1.088 2.91.832.092-.647.35-1.088.636-1.338-2.22-.253-4.555-1.113-4.555-4.951 0-1.093.39-1.988 1.029-2.688-.103-.253-.446-1.272.098-2.65 0 0 .84-.27 2.75 1.026A9.564 9.564 0 0112 6.844c.85.004 1.705.115 2.504.337 1.909-1.296 2.747-1.027 2.747-1.027.546 1.379.202 2.398.1 2.651.64.7 1.028 1.595 1.028 2.688 0 3.848-2.339 4.695-4.566 4.943.359.309.678.92.678 1.855 0 1.338-.012 2.419-.012 2.747 0 .268.18.58.688.482A10.019 10.019 0 0022 12.017C22 6.484 17.522 2 12 2z" clipRule="evenodd" />
                  </svg>
                  <h1 className="text-2xl font-semibold text-gray-900 dark:text-white">
                    {repo.owner}/{repo.name}
                  </h1>
                  <span className={`ml-3 inline-flex rounded-full px-2 py-1 text-xs font-semibold ${getStatusColor(repo.status)}`}>
                    {repo.status}
                  </span>
                </div>
                <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
                  {repo.url}
                </p>
              </div>
            </div>
            <div className="flex items-center space-x-3">
              <a
                href={`/jobs/new?repo=${repo.id}`}
                className="inline-flex items-center rounded-lg bg-indigo-600 px-4 py-2 text-sm font-medium text-white shadow-sm hover:bg-indigo-700"
              >
                <svg className="mr-2 h-4 w-4" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" d="M5.25 5.653c0-.856.917-1.398 1.667-.986l11.54 6.347a1.125 1.125 0 010 1.972l-11.54 6.347a1.125 1.125 0 01-1.667-.986V5.653z" />
                </svg>
                Generate Tests
              </a>
              <button
                onClick={handleDelete}
                disabled={deleting}
                className="inline-flex items-center rounded-lg bg-red-600 px-4 py-2 text-sm font-medium text-white shadow-sm hover:bg-red-700 disabled:opacity-50"
              >
                {deleting ? "Deleting..." : "Delete"}
              </button>
            </div>
          </div>
        </div>

        <div className="p-8">
          {/* Repository Info */}
          <div className="grid grid-cols-1 gap-6 lg:grid-cols-2">
            <div className="rounded-lg bg-white p-6 shadow-sm ring-1 ring-gray-200 dark:bg-gray-800 dark:ring-gray-700">
              <h2 className="text-lg font-medium text-gray-900 dark:text-white mb-4">Repository Details</h2>
              <dl className="space-y-3">
                <div className="flex justify-between">
                  <dt className="text-sm text-gray-500 dark:text-gray-400">Default Branch</dt>
                  <dd className="text-sm font-medium text-gray-900 dark:text-white">{repo.default_branch}</dd>
                </div>
                {repo.commit_sha && (
                  <div className="flex justify-between">
                    <dt className="text-sm text-gray-500 dark:text-gray-400">Last Commit</dt>
                    <dd className="text-sm font-mono text-gray-900 dark:text-white">{repo.commit_sha.slice(0, 8)}</dd>
                  </div>
                )}
                <div className="flex justify-between">
                  <dt className="text-sm text-gray-500 dark:text-gray-400">Created</dt>
                  <dd className="text-sm text-gray-900 dark:text-white">{new Date(repo.created_at).toLocaleDateString()}</dd>
                </div>
                <div className="flex justify-between">
                  <dt className="text-sm text-gray-500 dark:text-gray-400">Updated</dt>
                  <dd className="text-sm text-gray-900 dark:text-white">{new Date(repo.updated_at).toLocaleDateString()}</dd>
                </div>
              </dl>
            </div>

            <div className="rounded-lg bg-white p-6 shadow-sm ring-1 ring-gray-200 dark:bg-gray-800 dark:ring-gray-700">
              <h2 className="text-lg font-medium text-gray-900 dark:text-white mb-4">Quick Stats</h2>
              <div className="grid grid-cols-2 gap-4">
                <div className="rounded-lg bg-gray-50 p-4 dark:bg-gray-900">
                  <p className="text-2xl font-semibold text-indigo-600">{jobs.length}</p>
                  <p className="text-sm text-gray-500 dark:text-gray-400">Recent Jobs</p>
                </div>
                <div className="rounded-lg bg-gray-50 p-4 dark:bg-gray-900">
                  <p className="text-2xl font-semibold text-green-600">
                    {jobs.filter(j => j.status === "completed").length}
                  </p>
                  <p className="text-sm text-gray-500 dark:text-gray-400">Completed</p>
                </div>
                <div className="rounded-lg bg-gray-50 p-4 dark:bg-gray-900">
                  <p className="text-2xl font-semibold text-blue-600">
                    {jobs.filter(j => j.status === "running").length}
                  </p>
                  <p className="text-sm text-gray-500 dark:text-gray-400">Running</p>
                </div>
                <div className="rounded-lg bg-gray-50 p-4 dark:bg-gray-900">
                  <p className="text-2xl font-semibold text-red-600">
                    {jobs.filter(j => j.status === "failed").length}
                  </p>
                  <p className="text-sm text-gray-500 dark:text-gray-400">Failed</p>
                </div>
              </div>
            </div>
          </div>

          {/* Recent Jobs */}
          <div className="mt-8">
            <div className="flex items-center justify-between mb-4">
              <h2 className="text-lg font-medium text-gray-900 dark:text-white">Recent Jobs</h2>
              <a href={`/jobs?repo=${repo.id}`} className="text-sm text-indigo-600 hover:text-indigo-500">
                View all
              </a>
            </div>
            {jobs.length === 0 ? (
              <div className="rounded-lg bg-gray-50 p-8 text-center dark:bg-gray-900">
                <p className="text-gray-500 dark:text-gray-400">No jobs yet</p>
                <a
                  href={`/jobs/new?repo=${repo.id}`}
                  className="mt-2 inline-flex items-center text-indigo-600 hover:text-indigo-500"
                >
                  Start your first pipeline
                </a>
              </div>
            ) : (
              <div className="space-y-3">
                {jobs.map((job) => (
                  <a
                    key={job.id}
                    href={`/jobs/${job.id}`}
                    className="block rounded-lg bg-white p-4 shadow-sm ring-1 ring-gray-200 hover:ring-indigo-500 dark:bg-gray-800 dark:ring-gray-700 dark:hover:ring-indigo-400 transition-all"
                  >
                    <div className="flex items-center justify-between">
                      <div className="flex items-center">
                        <span className={`inline-flex rounded-full px-2 py-1 text-xs font-semibold ${getJobStatusColor(job.status)}`}>
                          {job.status}
                        </span>
                        <span className="ml-3 text-sm font-medium text-gray-900 dark:text-white">
                          {job.type}
                        </span>
                        <span className="ml-2 text-xs text-gray-500 dark:text-gray-400">
                          #{job.id.slice(0, 8)}
                        </span>
                      </div>
                      <span className="text-xs text-gray-500 dark:text-gray-400">
                        {new Date(job.created_at).toLocaleString()}
                      </span>
                    </div>
                    {job.error_message && (
                      <p className="mt-2 text-sm text-red-600 dark:text-red-400 truncate">
                        {job.error_message}
                      </p>
                    )}
                  </a>
                ))}
              </div>
            )}
          </div>

          {/* Recent Tests */}
          {tests.length > 0 && (
            <div className="mt-8">
              <div className="flex items-center justify-between mb-4">
                <h2 className="text-lg font-medium text-gray-900 dark:text-white">Recent Generated Tests</h2>
                <a href="/tests" className="text-sm text-indigo-600 hover:text-indigo-500">
                  View all
                </a>
              </div>
              <div className="overflow-hidden rounded-lg bg-white shadow-sm ring-1 ring-gray-200 dark:bg-gray-800 dark:ring-gray-700">
                <table className="min-w-full divide-y divide-gray-200 dark:divide-gray-700">
                  <thead className="bg-gray-50 dark:bg-gray-900">
                    <tr>
                      <th className="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400">
                        Test Name
                      </th>
                      <th className="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400">
                        Target
                      </th>
                      <th className="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400">
                        Status
                      </th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-gray-200 dark:divide-gray-700">
                    {tests.slice(0, 5).map((test) => (
                      <tr key={test.id} className="hover:bg-gray-50 dark:hover:bg-gray-700">
                        <td className="px-4 py-3 whitespace-nowrap text-sm font-medium text-gray-900 dark:text-white">
                          {test.name}
                        </td>
                        <td className="px-4 py-3 whitespace-nowrap text-sm text-gray-500 dark:text-gray-400">
                          {test.target_function || test.target_file}
                        </td>
                        <td className="px-4 py-3 whitespace-nowrap">
                          <span className={`inline-flex rounded-full px-2 py-1 text-xs font-semibold ${getJobStatusColor(test.status)}`}>
                            {test.status}
                          </span>
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
