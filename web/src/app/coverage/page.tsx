"use client";

import { useEffect, useState } from "react";
import Sidebar from "@/components/Sidebar";
import { api, CoverageSummary, CoverageSnapshot } from "@/lib/api";

export default function CoverageDashboardPage() {
  const [summary, setSummary] = useState<CoverageSummary | null>(null);
  const [snapshots, setSnapshots] = useState<CoverageSnapshot[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    loadData();
  }, []);

  async function loadData() {
    try {
      setLoading(true);
      const [summaryData, snapshotsData] = await Promise.all([
        api.getCoverageSummary().catch(() => null),
        api.listCoverageSnapshots(undefined, 20).catch(() => []),
      ]);
      setSummary(summaryData);
      setSnapshots(snapshotsData);
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load coverage data");
    } finally {
      setLoading(false);
    }
  }

  function getCoverageColor(percent: number): string {
    if (percent >= 80) return "text-green-600 dark:text-green-400";
    if (percent >= 60) return "text-yellow-600 dark:text-yellow-400";
    return "text-red-600 dark:text-red-400";
  }

  function getCoverageBgColor(percent: number): string {
    if (percent >= 80) return "bg-green-500";
    if (percent >= 60) return "bg-yellow-500";
    return "bg-red-500";
  }

  function getDeltaIcon(delta: number) {
    if (delta > 0) {
      return (
        <svg className="h-4 w-4 text-green-500" fill="none" viewBox="0 0 24 24" strokeWidth={2} stroke="currentColor">
          <path strokeLinecap="round" strokeLinejoin="round" d="M4.5 15.75l7.5-7.5 7.5 7.5" />
        </svg>
      );
    }
    if (delta < 0) {
      return (
        <svg className="h-4 w-4 text-red-500" fill="none" viewBox="0 0 24 24" strokeWidth={2} stroke="currentColor">
          <path strokeLinecap="round" strokeLinejoin="round" d="M19.5 8.25l-7.5 7.5-7.5-7.5" />
        </svg>
      );
    }
    return (
      <svg className="h-4 w-4 text-gray-400" fill="none" viewBox="0 0 24 24" strokeWidth={2} stroke="currentColor">
        <path strokeLinecap="round" strokeLinejoin="round" d="M18 12H6" />
      </svg>
    );
  }

  return (
    <div className="flex h-screen">
      <Sidebar />

      <main className="flex-1 overflow-y-auto">
        {/* Header */}
        <div className="border-b border-gray-200 bg-white px-8 py-6 dark:border-gray-700 dark:bg-gray-800">
          <h1 className="text-2xl font-semibold text-gray-900 dark:text-white">
            Coverage Dashboard
          </h1>
          <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
            Track code coverage trends across repositories
          </p>
        </div>

        <div className="p-8">
          {loading ? (
            <div className="flex items-center justify-center py-12">
              <div className="h-8 w-8 animate-spin rounded-full border-4 border-indigo-600 border-t-transparent" />
              <span className="ml-3 text-gray-500">Loading coverage data...</span>
            </div>
          ) : error ? (
            <div className="rounded-lg bg-red-50 p-4 dark:bg-red-900/20">
              <p className="text-sm text-red-800 dark:text-red-200">{error}</p>
              <button onClick={loadData} className="mt-2 text-sm text-red-600 hover:text-red-500">
                Try again
              </button>
            </div>
          ) : (
            <>
              {/* Summary Stats */}
              {summary && (
                <div className="grid grid-cols-1 gap-6 md:grid-cols-2 lg:grid-cols-4 mb-8">
                  <div className="rounded-lg bg-white p-6 shadow-sm ring-1 ring-gray-200 dark:bg-gray-800 dark:ring-gray-700">
                    <div className="flex items-center justify-between">
                      <p className="text-sm font-medium text-gray-500 dark:text-gray-400">Average Coverage</p>
                      {getDeltaIcon(summary.trend_delta)}
                    </div>
                    <p className={`mt-2 text-3xl font-semibold ${getCoverageColor(summary.avg_coverage)}`}>
                      {summary.avg_coverage.toFixed(1)}%
                    </p>
                    <p className="mt-1 text-xs text-gray-500">
                      {summary.trend_direction === "up" && `+${summary.trend_delta.toFixed(1)}% from last period`}
                      {summary.trend_direction === "down" && `${summary.trend_delta.toFixed(1)}% from last period`}
                      {summary.trend_direction === "stable" && "Stable"}
                    </p>
                  </div>

                  <div className="rounded-lg bg-white p-6 shadow-sm ring-1 ring-gray-200 dark:bg-gray-800 dark:ring-gray-700">
                    <p className="text-sm font-medium text-gray-500 dark:text-gray-400">Total Lines</p>
                    <p className="mt-2 text-3xl font-semibold text-gray-900 dark:text-white">
                      {summary.total_lines.toLocaleString()}
                    </p>
                    <p className="mt-1 text-xs text-gray-500">
                      {summary.total_covered.toLocaleString()} covered
                    </p>
                  </div>

                  <div className="rounded-lg bg-white p-6 shadow-sm ring-1 ring-gray-200 dark:bg-gray-800 dark:ring-gray-700">
                    <p className="text-sm font-medium text-gray-500 dark:text-gray-400">Repos Above 80%</p>
                    <p className="mt-2 text-3xl font-semibold text-green-600 dark:text-green-400">
                      {summary.repos_above_80}
                    </p>
                    <p className="mt-1 text-xs text-gray-500">
                      of {summary.total_repos} repositories
                    </p>
                  </div>

                  <div className="rounded-lg bg-white p-6 shadow-sm ring-1 ring-gray-200 dark:bg-gray-800 dark:ring-gray-700">
                    <p className="text-sm font-medium text-gray-500 dark:text-gray-400">Repos Below 50%</p>
                    <p className="mt-2 text-3xl font-semibold text-red-600 dark:text-red-400">
                      {summary.repos_below_50}
                    </p>
                    <p className="mt-1 text-xs text-gray-500">
                      need attention
                    </p>
                  </div>
                </div>
              )}

              {/* Coverage Distribution Bar */}
              {summary && summary.total_repos > 0 && (
                <div className="mb-8 rounded-lg bg-white p-6 shadow-sm ring-1 ring-gray-200 dark:bg-gray-800 dark:ring-gray-700">
                  <h2 className="text-lg font-medium text-gray-900 dark:text-white mb-4">Coverage Distribution</h2>
                  <div className="h-8 rounded-full bg-gray-200 dark:bg-gray-700 overflow-hidden flex">
                    {summary.repos_above_80 > 0 && (
                      <div
                        className="bg-green-500 flex items-center justify-center text-xs text-white font-medium"
                        style={{ width: `${(summary.repos_above_80 / summary.total_repos) * 100}%` }}
                      >
                        {summary.repos_above_80 > 0 && `${summary.repos_above_80}`}
                      </div>
                    )}
                    {(summary.total_repos - summary.repos_above_80 - summary.repos_below_50) > 0 && (
                      <div
                        className="bg-yellow-500 flex items-center justify-center text-xs text-white font-medium"
                        style={{ width: `${((summary.total_repos - summary.repos_above_80 - summary.repos_below_50) / summary.total_repos) * 100}%` }}
                      >
                        {(summary.total_repos - summary.repos_above_80 - summary.repos_below_50) > 0 &&
                          `${summary.total_repos - summary.repos_above_80 - summary.repos_below_50}`}
                      </div>
                    )}
                    {summary.repos_below_50 > 0 && (
                      <div
                        className="bg-red-500 flex items-center justify-center text-xs text-white font-medium"
                        style={{ width: `${(summary.repos_below_50 / summary.total_repos) * 100}%` }}
                      >
                        {summary.repos_below_50 > 0 && `${summary.repos_below_50}`}
                      </div>
                    )}
                  </div>
                  <div className="mt-2 flex justify-between text-xs text-gray-500">
                    <span className="flex items-center"><span className="w-3 h-3 rounded bg-green-500 mr-1"></span> 80%+</span>
                    <span className="flex items-center"><span className="w-3 h-3 rounded bg-yellow-500 mr-1"></span> 50-80%</span>
                    <span className="flex items-center"><span className="w-3 h-3 rounded bg-red-500 mr-1"></span> &lt;50%</span>
                  </div>
                </div>
              )}

              {/* Recent Snapshots */}
              <div className="rounded-lg bg-white shadow-sm ring-1 ring-gray-200 dark:bg-gray-800 dark:ring-gray-700">
                <div className="px-6 py-4 border-b border-gray-200 dark:border-gray-700">
                  <h2 className="text-lg font-medium text-gray-900 dark:text-white">Recent Coverage Snapshots</h2>
                </div>
                {snapshots.length === 0 ? (
                  <div className="p-8 text-center">
                    <svg className="mx-auto h-12 w-12 text-gray-400" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor">
                      <path strokeLinecap="round" strokeLinejoin="round" d="M3 13.125C3 12.504 3.504 12 4.125 12h2.25c.621 0 1.125.504 1.125 1.125v6.75C7.5 20.496 6.996 21 6.375 21h-2.25A1.125 1.125 0 013 19.875v-6.75zM9.75 8.625c0-.621.504-1.125 1.125-1.125h2.25c.621 0 1.125.504 1.125 1.125v11.25c0 .621-.504 1.125-1.125 1.125h-2.25a1.125 1.125 0 01-1.125-1.125V8.625zM16.5 4.125c0-.621.504-1.125 1.125-1.125h2.25C20.496 3 21 3.504 21 4.125v15.75c0 .621-.504 1.125-1.125 1.125h-2.25a1.125 1.125 0 01-1.125-1.125V4.125z" />
                    </svg>
                    <h3 className="mt-4 text-lg font-medium text-gray-900 dark:text-white">No coverage data</h3>
                    <p className="mt-2 text-sm text-gray-500 dark:text-gray-400">
                      Run test generation to collect coverage metrics.
                    </p>
                  </div>
                ) : (
                  <table className="min-w-full divide-y divide-gray-200 dark:divide-gray-700">
                    <thead className="bg-gray-50 dark:bg-gray-900">
                      <tr>
                        <th className="px-6 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400">
                          Repository
                        </th>
                        <th className="px-6 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400">
                          Branch
                        </th>
                        <th className="px-6 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400">
                          Coverage
                        </th>
                        <th className="px-6 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400">
                          Lines
                        </th>
                        <th className="px-6 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400">
                          Delta
                        </th>
                        <th className="px-6 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400">
                          Date
                        </th>
                      </tr>
                    </thead>
                    <tbody className="divide-y divide-gray-200 dark:divide-gray-700">
                      {snapshots.map((snap) => (
                        <tr key={snap.id} className="hover:bg-gray-50 dark:hover:bg-gray-700">
                          <td className="px-6 py-4 whitespace-nowrap">
                            <a href={`/repos/${snap.repository_id}`} className="text-sm font-medium text-indigo-600 hover:text-indigo-500">
                              {snap.repository_id.slice(0, 8)}...
                            </a>
                          </td>
                          <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500 dark:text-gray-400">
                            {snap.branch || "main"}
                          </td>
                          <td className="px-6 py-4 whitespace-nowrap">
                            <div className="flex items-center">
                              <div className="w-16 h-2 rounded-full bg-gray-200 dark:bg-gray-700 mr-2">
                                <div
                                  className={`h-full rounded-full ${getCoverageBgColor(snap.coverage_percent)}`}
                                  style={{ width: `${snap.coverage_percent}%` }}
                                />
                              </div>
                              <span className={`text-sm font-medium ${getCoverageColor(snap.coverage_percent)}`}>
                                {snap.coverage_percent.toFixed(1)}%
                              </span>
                            </div>
                          </td>
                          <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500 dark:text-gray-400">
                            {snap.covered_lines.toLocaleString()} / {snap.total_lines.toLocaleString()}
                          </td>
                          <td className="px-6 py-4 whitespace-nowrap">
                            <div className="flex items-center">
                              {getDeltaIcon(snap.coverage_delta)}
                              <span className={`ml-1 text-sm ${snap.coverage_delta > 0 ? "text-green-600" : snap.coverage_delta < 0 ? "text-red-600" : "text-gray-500"}`}>
                                {snap.coverage_delta > 0 ? "+" : ""}{snap.coverage_delta.toFixed(1)}%
                              </span>
                            </div>
                          </td>
                          <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500 dark:text-gray-400">
                            {new Date(snap.created_at).toLocaleDateString()}
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                )}
              </div>
            </>
          )}
        </div>
      </main>
    </div>
  );
}
