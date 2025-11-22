"use client";

import { useState, useEffect } from "react";
import Sidebar from "@/components/Sidebar";
import { api } from "@/lib/api";

export default function SettingsPage() {
  const [apiStatus, setApiStatus] = useState<"checking" | "online" | "offline">("checking");
  const [llmTier, setLlmTier] = useState(1);
  const [maxTests, setMaxTests] = useState(10);

  useEffect(() => {
    checkApiHealth();
  }, []);

  async function checkApiHealth() {
    try {
      await api.health();
      setApiStatus("online");
    } catch {
      setApiStatus("offline");
    }
  }

  return (
    <div className="flex h-screen">
      <Sidebar />

      <main className="flex-1 overflow-y-auto">
        {/* Header */}
        <div className="border-b border-gray-200 bg-white px-8 py-6 dark:border-gray-700 dark:bg-gray-800">
          <h1 className="text-2xl font-semibold text-gray-900 dark:text-white">Settings</h1>
          <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
            Configure QTest settings
          </p>
        </div>

        <div className="p-8 max-w-3xl">
          {/* API Status */}
          <div className="rounded-lg bg-white p-6 shadow-sm ring-1 ring-gray-200 dark:bg-gray-800 dark:ring-gray-700">
            <h2 className="text-lg font-medium text-gray-900 dark:text-white">API Connection</h2>
            <div className="mt-4 flex items-center">
              <div
                className={`h-3 w-3 rounded-full ${
                  apiStatus === "checking"
                    ? "bg-yellow-400 animate-pulse"
                    : apiStatus === "online"
                    ? "bg-green-400"
                    : "bg-red-400"
                }`}
              />
              <span className="ml-3 text-sm text-gray-700 dark:text-gray-300">
                {apiStatus === "checking"
                  ? "Checking connection..."
                  : apiStatus === "online"
                  ? "Connected to API server"
                  : "API server unavailable"}
              </span>
              <button
                onClick={checkApiHealth}
                className="ml-auto text-sm text-indigo-600 hover:text-indigo-700 dark:text-indigo-400"
              >
                Refresh
              </button>
            </div>
            <p className="mt-2 text-xs text-gray-500 dark:text-gray-400">
              API endpoint: {process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080"}
            </p>
          </div>

          {/* Generation Defaults */}
          <div className="mt-6 rounded-lg bg-white p-6 shadow-sm ring-1 ring-gray-200 dark:bg-gray-800 dark:ring-gray-700">
            <h2 className="text-lg font-medium text-gray-900 dark:text-white">
              Generation Defaults
            </h2>
            <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
              Default settings for test generation pipelines
            </p>

            <div className="mt-6 space-y-6">
              {/* Default LLM Tier */}
              <div>
                <label className="block text-sm font-medium text-gray-700 dark:text-gray-300">
                  Default LLM Tier
                </label>
                <select
                  value={llmTier}
                  onChange={(e) => setLlmTier(parseInt(e.target.value))}
                  className="mt-1 block w-full rounded-lg border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 dark:border-gray-600 dark:bg-gray-700 dark:text-white px-4 py-2"
                >
                  <option value={1}>Tier 1: Fast (qwen2.5-coder:7b)</option>
                  <option value={2}>Tier 2: Balanced (deepseek-coder-v2:16b)</option>
                  <option value={3}>Tier 3: Thorough (deepseek-coder-v2:16b)</option>
                </select>
              </div>

              {/* Default Max Tests */}
              <div>
                <label className="block text-sm font-medium text-gray-700 dark:text-gray-300">
                  Default Max Tests per File
                </label>
                <input
                  type="number"
                  value={maxTests}
                  onChange={(e) => setMaxTests(parseInt(e.target.value) || 10)}
                  min={1}
                  max={50}
                  className="mt-1 block w-32 rounded-lg border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 dark:border-gray-600 dark:bg-gray-700 dark:text-white px-4 py-2"
                />
              </div>
            </div>
          </div>

          {/* About */}
          <div className="mt-6 rounded-lg bg-white p-6 shadow-sm ring-1 ring-gray-200 dark:bg-gray-800 dark:ring-gray-700">
            <h2 className="text-lg font-medium text-gray-900 dark:text-white">About QTest</h2>
            <div className="mt-4 space-y-2 text-sm text-gray-600 dark:text-gray-400">
              <p>
                <span className="font-medium text-gray-900 dark:text-white">Version:</span> 0.1.0
              </p>
              <p>
                <span className="font-medium text-gray-900 dark:text-white">License:</span> MIT
              </p>
              <p className="pt-2">
                QTest is an AI-powered test generation platform that transforms any repository into
                a comprehensive test suite using LLMs.
              </p>
            </div>
            <div className="mt-4">
              <a
                href="https://github.com/QTest-hq/qtest"
                target="_blank"
                rel="noopener noreferrer"
                className="inline-flex items-center text-sm text-indigo-600 hover:text-indigo-700 dark:text-indigo-400"
              >
                <svg className="h-4 w-4 mr-1" fill="currentColor" viewBox="0 0 24 24">
                  <path
                    fillRule="evenodd"
                    d="M12 2C6.477 2 2 6.484 2 12.017c0 4.425 2.865 8.18 6.839 9.504.5.092.682-.217.682-.483 0-.237-.008-.868-.013-1.703-2.782.605-3.369-1.343-3.369-1.343-.454-1.158-1.11-1.466-1.11-1.466-.908-.62.069-.608.069-.608 1.003.07 1.531 1.032 1.531 1.032.892 1.53 2.341 1.088 2.91.832.092-.647.35-1.088.636-1.338-2.22-.253-4.555-1.113-4.555-4.951 0-1.093.39-1.988 1.029-2.688-.103-.253-.446-1.272.098-2.65 0 0 .84-.27 2.75 1.026A9.564 9.564 0 0112 6.844c.85.004 1.705.115 2.504.337 1.909-1.296 2.747-1.027 2.747-1.027.546 1.379.202 2.398.1 2.651.64.7 1.028 1.595 1.028 2.688 0 3.848-2.339 4.695-4.566 4.943.359.309.678.92.678 1.855 0 1.338-.012 2.419-.012 2.747 0 .268.18.58.688.482A10.019 10.019 0 0022 12.017C22 6.484 17.522 2 12 2z"
                    clipRule="evenodd"
                  />
                </svg>
                View on GitHub
              </a>
            </div>
          </div>
        </div>
      </main>
    </div>
  );
}
