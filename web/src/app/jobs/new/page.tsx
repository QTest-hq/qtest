"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import Sidebar from "@/components/Sidebar";
import { api } from "@/lib/api";

export default function NewPipelinePage() {
  const router = useRouter();
  const [repoUrl, setRepoUrl] = useState("");
  const [branch, setBranch] = useState("");
  const [maxTests, setMaxTests] = useState(10);
  const [llmTier, setLlmTier] = useState(1);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setLoading(true);
    setError(null);

    try {
      await api.startPipeline({
        repository_url: repoUrl,
        branch: branch || undefined,
        max_tests: maxTests,
        llm_tier: llmTier,
      });
      router.push("/jobs");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to start pipeline");
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="flex h-screen">
      <Sidebar />

      <main className="flex-1 overflow-y-auto">
        {/* Header */}
        <div className="border-b border-gray-200 bg-white px-8 py-6 dark:border-gray-700 dark:bg-gray-800">
          <h1 className="text-2xl font-semibold text-gray-900 dark:text-white">
            Start Pipeline
          </h1>
          <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
            Generate tests for a repository
          </p>
        </div>

        <div className="p-8">
          <form onSubmit={handleSubmit} className="max-w-2xl">
            <div className="space-y-6">
              {/* Repository URL */}
              <div>
                <label
                  htmlFor="repoUrl"
                  className="block text-sm font-medium text-gray-700 dark:text-gray-300"
                >
                  Repository URL *
                </label>
                <input
                  type="url"
                  id="repoUrl"
                  value={repoUrl}
                  onChange={(e) => setRepoUrl(e.target.value)}
                  placeholder="https://github.com/owner/repo"
                  required
                  className="mt-1 block w-full rounded-lg border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 dark:border-gray-600 dark:bg-gray-700 dark:text-white px-4 py-2"
                />
                <p className="mt-1 text-xs text-gray-500 dark:text-gray-400">
                  Public GitHub repository URL
                </p>
              </div>

              {/* Branch */}
              <div>
                <label
                  htmlFor="branch"
                  className="block text-sm font-medium text-gray-700 dark:text-gray-300"
                >
                  Branch
                </label>
                <input
                  type="text"
                  id="branch"
                  value={branch}
                  onChange={(e) => setBranch(e.target.value)}
                  placeholder="main"
                  className="mt-1 block w-full rounded-lg border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 dark:border-gray-600 dark:bg-gray-700 dark:text-white px-4 py-2"
                />
                <p className="mt-1 text-xs text-gray-500 dark:text-gray-400">
                  Leave empty to use default branch
                </p>
              </div>

              {/* Max Tests */}
              <div>
                <label
                  htmlFor="maxTests"
                  className="block text-sm font-medium text-gray-700 dark:text-gray-300"
                >
                  Max Tests per File
                </label>
                <input
                  type="number"
                  id="maxTests"
                  value={maxTests}
                  onChange={(e) => setMaxTests(parseInt(e.target.value) || 10)}
                  min={1}
                  max={50}
                  className="mt-1 block w-32 rounded-lg border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 dark:border-gray-600 dark:bg-gray-700 dark:text-white px-4 py-2"
                />
                <p className="mt-1 text-xs text-gray-500 dark:text-gray-400">
                  Maximum number of tests to generate per source file
                </p>
              </div>

              {/* LLM Tier */}
              <div>
                <label className="block text-sm font-medium text-gray-700 dark:text-gray-300">
                  LLM Tier
                </label>
                <div className="mt-2 space-y-2">
                  {[
                    { tier: 1, name: "Fast", desc: "qwen2.5-coder:7b - Quick generation" },
                    { tier: 2, name: "Balanced", desc: "deepseek-coder-v2:16b - Better quality" },
                    { tier: 3, name: "Thorough", desc: "deepseek-coder-v2:16b - Best quality" },
                  ].map((option) => (
                    <label
                      key={option.tier}
                      className={`flex cursor-pointer items-center rounded-lg border p-4 transition-colors ${
                        llmTier === option.tier
                          ? "border-indigo-600 bg-indigo-50 dark:border-indigo-500 dark:bg-indigo-900/20"
                          : "border-gray-200 hover:bg-gray-50 dark:border-gray-700 dark:hover:bg-gray-800"
                      }`}
                    >
                      <input
                        type="radio"
                        name="llmTier"
                        value={option.tier}
                        checked={llmTier === option.tier}
                        onChange={() => setLlmTier(option.tier)}
                        className="h-4 w-4 text-indigo-600 focus:ring-indigo-500"
                      />
                      <div className="ml-3">
                        <span className="block text-sm font-medium text-gray-900 dark:text-white">
                          Tier {option.tier}: {option.name}
                        </span>
                        <span className="block text-xs text-gray-500 dark:text-gray-400">
                          {option.desc}
                        </span>
                      </div>
                    </label>
                  ))}
                </div>
              </div>

              {/* Error */}
              {error && (
                <div className="rounded-lg bg-red-50 p-4 dark:bg-red-900/20">
                  <p className="text-sm text-red-700 dark:text-red-300">{error}</p>
                </div>
              )}

              {/* Submit */}
              <div className="flex items-center justify-end space-x-4">
                <a
                  href="/jobs"
                  className="rounded-lg px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-100 dark:text-gray-300 dark:hover:bg-gray-800"
                >
                  Cancel
                </a>
                <button
                  type="submit"
                  disabled={loading || !repoUrl}
                  className="inline-flex items-center rounded-lg bg-indigo-600 px-4 py-2 text-sm font-medium text-white shadow-sm hover:bg-indigo-700 disabled:opacity-50 disabled:cursor-not-allowed"
                >
                  {loading ? (
                    <>
                      <div className="mr-2 h-4 w-4 animate-spin rounded-full border-2 border-white border-t-transparent" />
                      Starting...
                    </>
                  ) : (
                    <>
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
                    </>
                  )}
                </button>
              </div>
            </div>
          </form>
        </div>
      </main>
    </div>
  );
}
