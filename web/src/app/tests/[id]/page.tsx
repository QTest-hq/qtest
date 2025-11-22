"use client";

import { useEffect, useState } from "react";
import { useParams } from "next/navigation";
import Sidebar from "@/components/Sidebar";
import IRSpecViewer from "@/components/IRSpecViewer";
import { api, GeneratedTest } from "@/lib/api";

type Tab = "code" | "irspec" | "dsl";

export default function TestDetailPage() {
  const params = useParams();
  const testId = params.id as string;

  const [test, setTest] = useState<GeneratedTest | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [activeTab, setActiveTab] = useState<Tab>("code");

  useEffect(() => {
    loadTest();
  }, [testId]);

  async function loadTest() {
    try {
      setLoading(true);
      const data = await api.getTest(testId);
      setTest(data);
      setError(null);
      // Default to IRSpec tab if available
      if (data.metadata?.irspec) {
        setActiveTab("irspec");
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load test");
    } finally {
      setLoading(false);
    }
  }

  async function handleAccept() {
    if (!test) return;
    try {
      await api.acceptTest(test.id);
      await loadTest();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to accept test");
    }
  }

  async function handleReject() {
    if (!test) return;
    const reason = prompt("Rejection reason (optional):");
    try {
      await api.rejectTest(test.id, reason || undefined);
      await loadTest();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to reject test");
    }
  }

  function getStatusColor(status: string): string {
    switch (status) {
      case "accepted":
      case "validated":
        return "bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200";
      case "fixed":
        return "bg-amber-100 text-amber-800 dark:bg-amber-900 dark:text-amber-200";
      case "rejected":
      case "compile_error":
      case "test_failure":
        return "bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200";
      case "pending":
        return "bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-200";
      default:
        return "bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-200";
    }
  }

  function getStatusLabel(status: string): string {
    switch (status) {
      case "validated":
        return "Validated";
      case "fixed":
        return "Auto-Fixed";
      case "compile_error":
        return "Compile Error";
      case "test_failure":
        return "Test Failure";
      default:
        return status;
    }
  }

  return (
    <div className="flex h-screen">
      <Sidebar />

      <main className="flex-1 overflow-y-auto">
        {/* Header */}
        <div className="border-b border-gray-200 bg-white px-8 py-6 dark:border-gray-700 dark:bg-gray-800">
          <div className="flex items-center justify-between">
            <div>
              <div className="flex items-center">
                <a
                  href="/tests"
                  className="text-sm text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200"
                >
                  Tests
                </a>
                <span className="mx-2 text-gray-400">/</span>
                <span className="text-sm text-gray-900 dark:text-white">{testId.slice(0, 8)}</span>
              </div>
              <h1 className="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">
                {test?.name || "Loading..."}
              </h1>
            </div>
            {test && test.status === "pending" && (
              <div className="flex space-x-3">
                <button
                  onClick={handleAccept}
                  className="px-4 py-2 bg-green-600 text-white rounded-lg hover:bg-green-700 transition-colors text-sm font-medium"
                >
                  Accept
                </button>
                <button
                  onClick={handleReject}
                  className="px-4 py-2 bg-red-600 text-white rounded-lg hover:bg-red-700 transition-colors text-sm font-medium"
                >
                  Reject
                </button>
              </div>
            )}
          </div>
        </div>

        <div className="p-8">
          {loading ? (
            <div className="flex items-center justify-center py-12">
              <div className="h-8 w-8 animate-spin rounded-full border-4 border-indigo-600 border-t-transparent" />
              <span className="ml-3 text-gray-500">Loading test...</span>
            </div>
          ) : error ? (
            <div className="rounded-lg bg-red-50 p-4 dark:bg-red-900/20">
              <p className="text-sm font-medium text-red-800 dark:text-red-200">{error}</p>
              <button
                onClick={loadTest}
                className="mt-2 text-sm text-red-600 hover:text-red-500 dark:text-red-400"
              >
                Try again
              </button>
            </div>
          ) : test ? (
            <div className="space-y-6">
              {/* Meta info */}
              <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
                <div className="bg-white dark:bg-gray-800 rounded-lg p-4 shadow-sm ring-1 ring-gray-200 dark:ring-gray-700">
                  <p className="text-xs text-gray-500 dark:text-gray-400 uppercase">Status</p>
                  <span
                    className={`inline-flex mt-1 px-2 py-0.5 text-xs font-semibold rounded ${getStatusColor(test.status)}`}
                  >
                    {getStatusLabel(test.status)}
                  </span>
                  {test.status === "fixed" && (
                    <p className="mt-1 text-xs text-gray-500 dark:text-gray-400">
                      Fixed by LLM
                    </p>
                  )}
                </div>
                <div className="bg-white dark:bg-gray-800 rounded-lg p-4 shadow-sm ring-1 ring-gray-200 dark:ring-gray-700">
                  <p className="text-xs text-gray-500 dark:text-gray-400 uppercase">Framework</p>
                  <p className="mt-1 font-medium text-gray-900 dark:text-white">
                    {test.framework || "N/A"}
                  </p>
                </div>
                <div className="bg-white dark:bg-gray-800 rounded-lg p-4 shadow-sm ring-1 ring-gray-200 dark:ring-gray-700">
                  <p className="text-xs text-gray-500 dark:text-gray-400 uppercase">Target</p>
                  <p className="mt-1 font-medium text-gray-900 dark:text-white truncate">
                    {test.target_function || test.target_file}
                  </p>
                </div>
                <div className="bg-white dark:bg-gray-800 rounded-lg p-4 shadow-sm ring-1 ring-gray-200 dark:ring-gray-700">
                  <p className="text-xs text-gray-500 dark:text-gray-400 uppercase">
                    Mutation Score
                  </p>
                  <p className="mt-1 font-medium text-gray-900 dark:text-white">
                    {test.mutation_score !== undefined
                      ? `${Math.round(test.mutation_score * 100)}%`
                      : "N/A"}
                  </p>
                </div>
              </div>

              {/* Tabs */}
              <div className="bg-white dark:bg-gray-800 rounded-lg shadow-sm ring-1 ring-gray-200 dark:ring-gray-700">
                <div className="border-b border-gray-200 dark:border-gray-700">
                  <nav className="flex -mb-px">
                    <button
                      onClick={() => setActiveTab("code")}
                      className={`px-6 py-3 text-sm font-medium border-b-2 transition-colors ${
                        activeTab === "code"
                          ? "border-indigo-600 text-indigo-600 dark:border-indigo-400 dark:text-indigo-400"
                          : "border-transparent text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200"
                      }`}
                    >
                      Generated Code
                    </button>
                    {test.metadata?.irspec && (
                      <button
                        onClick={() => setActiveTab("irspec")}
                        className={`px-6 py-3 text-sm font-medium border-b-2 transition-colors ${
                          activeTab === "irspec"
                            ? "border-indigo-600 text-indigo-600 dark:border-indigo-400 dark:text-indigo-400"
                            : "border-transparent text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200"
                        }`}
                      >
                        IRSpec
                        <span className="ml-1.5 px-1.5 py-0.5 text-xs bg-indigo-100 text-indigo-700 dark:bg-indigo-900 dark:text-indigo-300 rounded">
                          Given-When-Then
                        </span>
                      </button>
                    )}
                    {test.dsl !== undefined && test.dsl !== null && (
                      <button
                        onClick={() => setActiveTab("dsl")}
                        className={`px-6 py-3 text-sm font-medium border-b-2 transition-colors ${
                          activeTab === "dsl"
                            ? "border-indigo-600 text-indigo-600 dark:border-indigo-400 dark:text-indigo-400"
                            : "border-transparent text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200"
                        }`}
                      >
                        DSL
                      </button>
                    )}
                  </nav>
                </div>

                <div className="p-6">
                  {activeTab === "code" && (
                    <div>
                      {test.generated_code ? (
                        <pre className="p-4 bg-gray-900 text-gray-100 rounded-lg overflow-auto text-sm font-mono">
                          {test.generated_code}
                        </pre>
                      ) : (
                        <p className="text-gray-500 dark:text-gray-400">
                          No generated code available
                        </p>
                      )}
                    </div>
                  )}

                  {activeTab === "irspec" && test.metadata?.irspec && (
                    <IRSpecViewer irspec={test.metadata.irspec} />
                  )}

                  {activeTab === "dsl" && test.dsl !== undefined && test.dsl !== null && (
                    <pre className="p-4 bg-gray-900 text-gray-100 rounded-lg overflow-auto text-sm font-mono">
                      {typeof test.dsl === "string"
                        ? test.dsl
                        : JSON.stringify(test.dsl, null, 2)}
                    </pre>
                  )}
                </div>
              </div>

              {/* Timestamps */}
              <div className="text-xs text-gray-500 dark:text-gray-400">
                Created: {new Date(test.created_at).toLocaleString()}
                <span className="mx-2">|</span>
                Updated: {new Date(test.updated_at).toLocaleString()}
              </div>
            </div>
          ) : (
            <p className="text-gray-500">Test not found</p>
          )}
        </div>
      </main>
    </div>
  );
}
