"use client";

import { useState } from "react";
import type { IRTestSuite, IRTestCase, IRVariable, IRAssertion } from "@/lib/api";

interface IRSpecViewerProps {
  irspec: IRTestSuite;
}

function formatValue(value: unknown): string {
  if (value === null) return "null";
  if (value === undefined) return "undefined";
  if (typeof value === "string") return `"${value}"`;
  if (typeof value === "object") return JSON.stringify(value, null, 2);
  return String(value);
}

function getTypeColor(type: string): string {
  switch (type) {
    case "int":
    case "float":
      return "text-blue-600 dark:text-blue-400";
    case "string":
      return "text-green-600 dark:text-green-400";
    case "bool":
      return "text-purple-600 dark:text-purple-400";
    case "null":
      return "text-gray-500 dark:text-gray-400";
    case "array":
    case "object":
      return "text-orange-600 dark:text-orange-400";
    default:
      return "text-gray-700 dark:text-gray-300";
  }
}

function getAssertionIcon(type: string): string {
  switch (type) {
    case "equals":
      return "=";
    case "not_equals":
      return "!=";
    case "greater_than":
      return ">";
    case "less_than":
      return "<";
    case "contains":
      return "in";
    case "throws":
      return "!";
    case "truthy":
      return "T";
    case "falsy":
      return "F";
    case "nil":
    case "not_nil":
      return "N";
    default:
      return "?";
  }
}

function getTagColor(tag: string): string {
  switch (tag) {
    case "happy_path":
      return "bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200";
    case "edge_case":
      return "bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-200";
    case "error_handling":
    case "error":
      return "bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200";
    case "boundary":
      return "bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200";
    default:
      return "bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-200";
  }
}

function VariableRow({ variable }: { variable: IRVariable }) {
  return (
    <div className="flex items-start py-1.5 px-3 bg-gray-50 dark:bg-gray-800 rounded-md">
      <span className="font-mono text-sm font-medium text-gray-900 dark:text-white min-w-[80px]">
        {variable.name}
      </span>
      <span className="mx-2 text-gray-400">=</span>
      <span className={`font-mono text-sm ${getTypeColor(variable.type)}`}>
        {formatValue(variable.value)}
      </span>
      <span className="ml-auto text-xs text-gray-400 uppercase">{variable.type}</span>
    </div>
  );
}

function AssertionRow({ assertion }: { assertion: IRAssertion }) {
  return (
    <div className="flex items-center py-1.5 px-3 bg-gray-50 dark:bg-gray-800 rounded-md">
      <span className="w-6 h-6 flex items-center justify-center rounded bg-indigo-100 text-indigo-700 dark:bg-indigo-900 dark:text-indigo-300 text-xs font-bold mr-2">
        {getAssertionIcon(assertion.type)}
      </span>
      <span className="font-mono text-sm text-gray-900 dark:text-white">
        {assertion.actual}
      </span>
      <span className="mx-2 text-xs text-gray-500">{assertion.type.replace(/_/g, " ")}</span>
      {assertion.expected !== undefined && (
        <span className="font-mono text-sm text-green-600 dark:text-green-400">
          {formatValue(assertion.expected)}
        </span>
      )}
    </div>
  );
}

function TestCaseCard({ testCase, index }: { testCase: IRTestCase; index: number }) {
  const [isExpanded, setIsExpanded] = useState(true);

  return (
    <div className="border border-gray-200 dark:border-gray-700 rounded-lg overflow-hidden">
      <button
        onClick={() => setIsExpanded(!isExpanded)}
        className="w-full px-4 py-3 flex items-center justify-between bg-white dark:bg-gray-800 hover:bg-gray-50 dark:hover:bg-gray-750 transition-colors"
      >
        <div className="flex items-center">
          <span className="w-6 h-6 flex items-center justify-center rounded-full bg-indigo-600 text-white text-xs font-bold mr-3">
            {index + 1}
          </span>
          <div className="text-left">
            <h4 className="font-medium text-gray-900 dark:text-white">
              {testCase.name.replace(/_/g, " ")}
            </h4>
            {testCase.description && (
              <p className="text-sm text-gray-500 dark:text-gray-400">{testCase.description}</p>
            )}
          </div>
        </div>
        <div className="flex items-center">
          {testCase.tags?.map((tag) => (
            <span
              key={tag}
              className={`ml-2 px-2 py-0.5 text-xs font-medium rounded ${getTagColor(tag)}`}
            >
              {tag.replace(/_/g, " ")}
            </span>
          ))}
          <svg
            className={`ml-3 w-5 h-5 text-gray-400 transition-transform ${isExpanded ? "rotate-180" : ""}`}
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
          </svg>
        </div>
      </button>

      {isExpanded && (
        <div className="p-4 space-y-4 bg-white dark:bg-gray-800 border-t border-gray-200 dark:border-gray-700">
          {/* Given */}
          <div>
            <div className="flex items-center mb-2">
              <span className="px-2 py-0.5 text-xs font-bold uppercase bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200 rounded">
                Given
              </span>
              <span className="ml-2 text-xs text-gray-500">Setup variables</span>
            </div>
            <div className="space-y-1.5">
              {testCase.given.map((variable, i) => (
                <VariableRow key={i} variable={variable} />
              ))}
            </div>
          </div>

          {/* When */}
          <div>
            <div className="flex items-center mb-2">
              <span className="px-2 py-0.5 text-xs font-bold uppercase bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-200 rounded">
                When
              </span>
              <span className="ml-2 text-xs text-gray-500">Action</span>
            </div>
            <div className="py-2 px-3 bg-gray-50 dark:bg-gray-800 rounded-md">
              <code className="font-mono text-sm text-gray-900 dark:text-white">
                {testCase.when.call}
              </code>
              {testCase.when.args && testCase.when.args.length > 0 && (
                <div className="mt-1 text-xs text-gray-500">
                  args: {testCase.when.args.join(", ")}
                </div>
              )}
            </div>
          </div>

          {/* Then */}
          <div>
            <div className="flex items-center mb-2">
              <span className="px-2 py-0.5 text-xs font-bold uppercase bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200 rounded">
                Then
              </span>
              <span className="ml-2 text-xs text-gray-500">Assertions</span>
            </div>
            <div className="space-y-1.5">
              {testCase.then.map((assertion, i) => (
                <AssertionRow key={i} assertion={assertion} />
              ))}
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

export default function IRSpecViewer({ irspec }: IRSpecViewerProps) {
  const [showRawJson, setShowRawJson] = useState(false);

  return (
    <div className="space-y-4">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h3 className="text-lg font-semibold text-gray-900 dark:text-white">
            IRSpec: {irspec.function_name}
          </h3>
          {irspec.description && (
            <p className="text-sm text-gray-500 dark:text-gray-400">{irspec.description}</p>
          )}
        </div>
        <div className="flex items-center space-x-2">
          <span className="px-2 py-1 text-xs font-medium bg-indigo-100 text-indigo-800 dark:bg-indigo-900 dark:text-indigo-200 rounded">
            {irspec.tests.length} test{irspec.tests.length !== 1 ? "s" : ""}
          </span>
          <button
            onClick={() => setShowRawJson(!showRawJson)}
            className="px-2 py-1 text-xs font-medium text-gray-600 hover:text-gray-900 dark:text-gray-400 dark:hover:text-white border border-gray-300 dark:border-gray-600 rounded transition-colors"
          >
            {showRawJson ? "Visual" : "JSON"}
          </button>
        </div>
      </div>

      {showRawJson ? (
        <pre className="p-4 bg-gray-900 text-green-400 rounded-lg overflow-auto text-sm font-mono">
          {JSON.stringify(irspec, null, 2)}
        </pre>
      ) : (
        <div className="space-y-3">
          {irspec.tests.map((testCase, index) => (
            <TestCaseCard key={index} testCase={testCase} index={index} />
          ))}
        </div>
      )}
    </div>
  );
}
