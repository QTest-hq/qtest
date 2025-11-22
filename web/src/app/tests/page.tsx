"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import Sidebar from "@/components/Sidebar";
import { api, MutationRun, GeneratedTest } from "@/lib/api";

type Tab = "tests" | "mutations";

export default function TestsPage() {
  const [activeTab, setActiveTab] = useState<Tab>("tests");
  const [tests, setTests] = useState<GeneratedTest[]>([]);
  const [runs, setRuns] = useState<MutationRun[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (activeTab === "tests") {
      loadTests();
    } else {
      loadRuns();
    }
  }, [activeTab]);

  async function loadTests() {
    try {
      setLoading(true);
      const data = await api.listTests({ limit: 50 });
      setTests(data);
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load tests");
    } finally {
      setLoading(false);
    }
  }

  async function loadRuns() {
    try {
      setLoading(true);
      const data = await api.listMutationRuns({ limit: 50 });
      setRuns(data);
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load tests");
    } finally {
      setLoading(false);
    }
  }

  function getStatusColor(status: string): string {
    switch (status) {
      case "completed":
      case "accepted":
      case "validated":
      case "fixed":
        return "bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200";
      case "running":
        return "bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200";
      case "pending":
        return "bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-200";
      case "failed":
      case "rejected":
      case "compile_error":
      case "test_failure":
        return "bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200";
      default:
        return "bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-200";
    }
  }

  function getQualityColor(quality?: string): string {
    switch (quality) {
      case "excellent":
        return "text-green-600 dark:text-green-400";
      case "good":
        return "text-blue-600 dark:text-blue-400";
      case "acceptable":
        return "text-yellow-600 dark:text-yellow-400";
      case "poor":
        return "text-red-600 dark:text-red-400";
      default:
        return "text-gray-500";
    }
  }

  return (
    <div className="flex h-screen">
      <Sidebar />

      <main className="flex-1 overflow-y-auto">
        {/* Header */}
        <div className="border-b border-gray-200 bg-white px-8 py-6 dark:border-gray-700 dark:bg-gray-800">
          <h1 className="text-2xl font-semibold text-gray-900 dark:text-white">Tests</h1>
          <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
            Generated tests with IRSpec and mutation scores
          </p>
        </div>

        {/* Tabs */}
        <div className="border-b border-gray-200 bg-white dark:border-gray-700 dark:bg-gray-800">
          <nav className="flex px-8 -mb-px">
            <button
              onClick={() => setActiveTab("tests")}
              className={`px-4 py-3 text-sm font-medium border-b-2 transition-colors ${
                activeTab === "tests"
                  ? "border-indigo-600 text-indigo-600 dark:border-indigo-400 dark:text-indigo-400"
                  : "border-transparent text-gray-500 hover:text-gray-700 dark:text-gray-400"
              }`}
            >
              Generated Tests
            </button>
            <button
              onClick={() => setActiveTab("mutations")}
              className={`ml-4 px-4 py-3 text-sm font-medium border-b-2 transition-colors ${
                activeTab === "mutations"
                  ? "border-indigo-600 text-indigo-600 dark:border-indigo-400 dark:text-indigo-400"
                  : "border-transparent text-gray-500 hover:text-gray-700 dark:text-gray-400"
              }`}
            >
              Mutation Runs
            </button>
          </nav>
        </div>

        <div className="p-8">
          {loading ? (
            <div className="flex items-center justify-center py-12">
              <div className="h-8 w-8 animate-spin rounded-full border-4 border-indigo-600 border-t-transparent" />
              <span className="ml-3 text-gray-500">Loading...</span>
            </div>
          ) : error ? (
            <div className="rounded-lg bg-red-50 p-4 dark:bg-red-900/20">
              <p className="text-sm font-medium text-red-800 dark:text-red-200">{error}</p>
              <button
                onClick={activeTab === "tests" ? loadTests : loadRuns}
                className="mt-2 text-sm text-red-600 hover:text-red-500 dark:text-red-400"
              >
                Try again
              </button>
            </div>
          ) : activeTab === "tests" ? (
            tests.length === 0 ? (
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
                    d="M9.75 3.104v5.714a2.25 2.25 0 01-.659 1.591L5 14.5M9.75 3.104c-.251.023-.501.05-.75.082m.75-.082a24.301 24.301 0 014.5 0m0 0v5.714c0 .597.237 1.17.659 1.591L19.8 15.3M14.25 3.104c.251.023.501.05.75.082M19.8 15.3l-1.57.393A9.065 9.065 0 0112 15a9.065 9.065 0 01-6.23.693L5 14.5m14.8.8l1.402 1.402c1.232 1.232.65 3.318-1.067 3.611A48.309 48.309 0 0112 21c-2.773 0-5.491-.235-8.135-.687-1.718-.293-2.3-2.379-1.067-3.61L5 14.5"
                  />
                </svg>
                <h3 className="mt-4 text-lg font-medium text-gray-900 dark:text-white">
                  No generated tests yet
                </h3>
                <p className="mt-2 text-sm text-gray-500 dark:text-gray-400">
                  Start a pipeline to generate tests with IRSpec.
                </p>
                <Link
                  href="/jobs/new"
                  className="mt-4 inline-flex items-center rounded-lg bg-indigo-600 px-4 py-2 text-sm font-medium text-white shadow-sm hover:bg-indigo-700"
                >
                  Start Pipeline
                </Link>
              </div>
            ) : (
              <div className="space-y-3">
                {tests.map((test) => (
                  <Link
                    key={test.id}
                    href={`/tests/${test.id}`}
                    className="block rounded-lg bg-white p-5 shadow-sm ring-1 ring-gray-200 dark:bg-gray-800 dark:ring-gray-700 hover:ring-indigo-500 dark:hover:ring-indigo-400 transition-all"
                  >
                    <div className="flex items-start justify-between">
                      <div className="flex-1 min-w-0">
                        <div className="flex items-center gap-2">
                          <span
                            className={`inline-flex rounded-full px-2 py-0.5 text-xs font-semibold ${getStatusColor(test.status)}`}
                          >
                            {test.status}
                          </span>
                          {test.status === "fixed" && (
                            <span className="inline-flex px-2 py-0.5 text-xs font-medium bg-amber-100 text-amber-700 dark:bg-amber-900 dark:text-amber-300 rounded">
                              Auto-Fixed
                            </span>
                          )}
                          {test.status === "validated" && (
                            <span className="inline-flex px-2 py-0.5 text-xs font-medium bg-emerald-100 text-emerald-700 dark:bg-emerald-900 dark:text-emerald-300 rounded">
                              Verified
                            </span>
                          )}
                          {test.metadata?.irspec_mode && (
                            <span className="inline-flex px-2 py-0.5 text-xs font-medium bg-indigo-100 text-indigo-700 dark:bg-indigo-900 dark:text-indigo-300 rounded">
                              IRSpec
                            </span>
                          )}
                          {test.framework && (
                            <span className="text-xs text-gray-500 dark:text-gray-400">
                              {test.framework}
                            </span>
                          )}
                        </div>
                        <h3 className="mt-2 text-sm font-medium text-gray-900 dark:text-white truncate">
                          {test.name}
                        </h3>
                        <p className="text-xs text-gray-500 dark:text-gray-400 truncate">
                          {test.target_function || test.target_file}
                        </p>
                      </div>
                      <div className="ml-4 flex items-center">
                        {test.mutation_score !== undefined && (
                          <div className="text-right mr-4">
                            <p className="text-lg font-bold text-gray-900 dark:text-white">
                              {Math.round(test.mutation_score * 100)}%
                            </p>
                            <p className="text-xs text-gray-500">mutation</p>
                          </div>
                        )}
                        <svg
                          className="w-5 h-5 text-gray-400"
                          fill="none"
                          viewBox="0 0 24 24"
                          stroke="currentColor"
                        >
                          <path
                            strokeLinecap="round"
                            strokeLinejoin="round"
                            strokeWidth={2}
                            d="M9 5l7 7-7 7"
                          />
                        </svg>
                      </div>
                    </div>
                    <div className="mt-2 text-xs text-gray-400">
                      {new Date(test.created_at).toLocaleString()}
                    </div>
                  </Link>
                ))}
              </div>
            )
          ) : runs.length === 0 ? (
            <div className="text-center py-12">
              <h3 className="text-lg font-medium text-gray-900 dark:text-white">
                No mutation runs yet
              </h3>
              <p className="mt-2 text-sm text-gray-500 dark:text-gray-400">
                Mutation testing will run automatically after test generation.
              </p>
            </div>
          ) : (
            <div className="space-y-4">
              {runs.map((run) => (
                <div
                  key={run.id}
                  className="rounded-lg bg-white p-6 shadow-sm ring-1 ring-gray-200 dark:bg-gray-800 dark:ring-gray-700"
                >
                  <div className="flex items-start justify-between">
                    <div>
                      <div className="flex items-center">
                        <span
                          className={`inline-flex rounded-full px-2 py-1 text-xs font-semibold ${getStatusColor(run.status)}`}
                        >
                          {run.status}
                        </span>
                        <span className="ml-2 text-xs text-gray-500 dark:text-gray-400">
                          #{run.id.slice(0, 8)}
                        </span>
                      </div>
                      <p className="mt-2 text-sm font-medium text-gray-900 dark:text-white">
                        {run.source_file}
                      </p>
                      <p className="text-sm text-gray-500 dark:text-gray-400">{run.test_file}</p>
                    </div>

                    {run.result && (
                      <div className="text-right">
                        <p className={`text-2xl font-bold ${getQualityColor(run.result.quality)}`}>
                          {Math.round(run.result.score * 100)}%
                        </p>
                        <p className="text-xs text-gray-500 dark:text-gray-400 capitalize">
                          {run.result.quality}
                        </p>
                      </div>
                    )}
                  </div>

                  {run.result && (
                    <div className="mt-4 grid grid-cols-4 gap-4 text-center">
                      <div>
                        <p className="text-lg font-semibold text-gray-900 dark:text-white">
                          {run.result.total}
                        </p>
                        <p className="text-xs text-gray-500">Total</p>
                      </div>
                      <div>
                        <p className="text-lg font-semibold text-green-600 dark:text-green-400">
                          {run.result.killed}
                        </p>
                        <p className="text-xs text-gray-500">Killed</p>
                      </div>
                      <div>
                        <p className="text-lg font-semibold text-red-600 dark:text-red-400">
                          {run.result.survived}
                        </p>
                        <p className="text-xs text-gray-500">Survived</p>
                      </div>
                      <div>
                        <p className="text-lg font-semibold text-yellow-600 dark:text-yellow-400">
                          {run.result.timeout}
                        </p>
                        <p className="text-xs text-gray-500">Timeout</p>
                      </div>
                    </div>
                  )}

                  <div className="mt-4 text-xs text-gray-500 dark:text-gray-400">
                    Created: {new Date(run.created_at).toLocaleString()}
                    {run.completed_at && (
                      <span className="ml-4">
                        Completed: {new Date(run.completed_at).toLocaleString()}
                      </span>
                    )}
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
