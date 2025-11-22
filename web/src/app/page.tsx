import Sidebar from "@/components/Sidebar";

const stats = [
  { name: "Total Repositories", value: "0", change: "+0 this week" },
  { name: "Tests Generated", value: "0", change: "+0 this week" },
  { name: "Jobs Running", value: "0", change: "0 pending" },
  { name: "Mutation Score", value: "N/A", change: "No data yet" },
];

const recentActivity = [
  { type: "info", message: "Welcome to QTest! Add a repository to get started.", time: "Just now" },
];

export default function Dashboard() {
  return (
    <div className="flex h-screen">
      <Sidebar />

      <main className="flex-1 overflow-y-auto">
        {/* Header */}
        <div className="border-b border-gray-200 bg-white px-8 py-6 dark:border-gray-700 dark:bg-gray-800">
          <h1 className="text-2xl font-semibold text-gray-900 dark:text-white">Dashboard</h1>
          <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
            AI-powered test generation overview
          </p>
        </div>

        <div className="p-8">
          {/* Stats */}
          <div className="grid grid-cols-1 gap-6 sm:grid-cols-2 lg:grid-cols-4">
            {stats.map((stat) => (
              <div
                key={stat.name}
                className="rounded-lg bg-white p-6 shadow-sm ring-1 ring-gray-200 dark:bg-gray-800 dark:ring-gray-700"
              >
                <p className="text-sm font-medium text-gray-500 dark:text-gray-400">{stat.name}</p>
                <p className="mt-2 text-3xl font-semibold text-gray-900 dark:text-white">{stat.value}</p>
                <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">{stat.change}</p>
              </div>
            ))}
          </div>

          {/* Quick Actions */}
          <div className="mt-8">
            <h2 className="text-lg font-medium text-gray-900 dark:text-white">Quick Actions</h2>
            <div className="mt-4 grid grid-cols-1 gap-4 sm:grid-cols-3">
              <a
                href="/repos/new"
                className="flex items-center rounded-lg bg-indigo-600 px-6 py-4 text-white shadow-sm hover:bg-indigo-700 transition-colors"
              >
                <svg className="h-6 w-6 mr-3" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" d="M12 4.5v15m7.5-7.5h-15" />
                </svg>
                <div>
                  <p className="font-medium">Add Repository</p>
                  <p className="text-sm text-indigo-200">Connect a GitHub repo</p>
                </div>
              </a>

              <a
                href="/jobs/new"
                className="flex items-center rounded-lg bg-white px-6 py-4 shadow-sm ring-1 ring-gray-200 hover:bg-gray-50 transition-colors dark:bg-gray-800 dark:ring-gray-700 dark:hover:bg-gray-700"
              >
                <svg className="h-6 w-6 mr-3 text-gray-400" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" d="M5.25 5.653c0-.856.917-1.398 1.667-.986l11.54 6.347a1.125 1.125 0 0 1 0 1.972l-11.54 6.347a1.125 1.125 0 0 1-1.667-.986V5.653Z" />
                </svg>
                <div>
                  <p className="font-medium text-gray-900 dark:text-white">Start Pipeline</p>
                  <p className="text-sm text-gray-500 dark:text-gray-400">Generate tests for a repo</p>
                </div>
              </a>

              <a
                href="https://github.com/QTest-hq/qtest"
                target="_blank"
                rel="noopener noreferrer"
                className="flex items-center rounded-lg bg-white px-6 py-4 shadow-sm ring-1 ring-gray-200 hover:bg-gray-50 transition-colors dark:bg-gray-800 dark:ring-gray-700 dark:hover:bg-gray-700"
              >
                <svg className="h-6 w-6 mr-3 text-gray-400" fill="currentColor" viewBox="0 0 24 24">
                  <path fillRule="evenodd" d="M12 2C6.477 2 2 6.484 2 12.017c0 4.425 2.865 8.18 6.839 9.504.5.092.682-.217.682-.483 0-.237-.008-.868-.013-1.703-2.782.605-3.369-1.343-3.369-1.343-.454-1.158-1.11-1.466-1.11-1.466-.908-.62.069-.608.069-.608 1.003.07 1.531 1.032 1.531 1.032.892 1.53 2.341 1.088 2.91.832.092-.647.35-1.088.636-1.338-2.22-.253-4.555-1.113-4.555-4.951 0-1.093.39-1.988 1.029-2.688-.103-.253-.446-1.272.098-2.65 0 0 .84-.27 2.75 1.026A9.564 9.564 0 0112 6.844c.85.004 1.705.115 2.504.337 1.909-1.296 2.747-1.027 2.747-1.027.546 1.379.202 2.398.1 2.651.64.7 1.028 1.595 1.028 2.688 0 3.848-2.339 4.695-4.566 4.943.359.309.678.92.678 1.855 0 1.338-.012 2.419-.012 2.747 0 .268.18.58.688.482A10.019 10.019 0 0022 12.017C22 6.484 17.522 2 12 2z" clipRule="evenodd" />
                </svg>
                <div>
                  <p className="font-medium text-gray-900 dark:text-white">View on GitHub</p>
                  <p className="text-sm text-gray-500 dark:text-gray-400">Star the project</p>
                </div>
              </a>
            </div>
          </div>

          {/* Recent Activity */}
          <div className="mt-8">
            <h2 className="text-lg font-medium text-gray-900 dark:text-white">Recent Activity</h2>
            <div className="mt-4 rounded-lg bg-white shadow-sm ring-1 ring-gray-200 dark:bg-gray-800 dark:ring-gray-700">
              <ul className="divide-y divide-gray-200 dark:divide-gray-700">
                {recentActivity.map((activity, idx) => (
                  <li key={idx} className="px-6 py-4">
                    <div className="flex items-center">
                      <div className={`h-2 w-2 rounded-full ${
                        activity.type === "success" ? "bg-green-400" :
                        activity.type === "error" ? "bg-red-400" :
                        "bg-blue-400"
                      }`} />
                      <p className="ml-3 text-sm text-gray-700 dark:text-gray-300">{activity.message}</p>
                      <span className="ml-auto text-sm text-gray-400">{activity.time}</span>
                    </div>
                  </li>
                ))}
              </ul>
            </div>
          </div>

          {/* API Status */}
          <div className="mt-8">
            <h2 className="text-lg font-medium text-gray-900 dark:text-white">System Status</h2>
            <div className="mt-4 rounded-lg bg-white p-6 shadow-sm ring-1 ring-gray-200 dark:bg-gray-800 dark:ring-gray-700">
              <div className="flex items-center justify-between">
                <div className="flex items-center">
                  <div className="h-3 w-3 rounded-full bg-yellow-400 animate-pulse" />
                  <span className="ml-3 text-sm font-medium text-gray-700 dark:text-gray-300">
                    API Server
                  </span>
                </div>
                <span className="text-sm text-gray-500">Checking...</span>
              </div>
              <p className="mt-2 text-xs text-gray-500 dark:text-gray-400">
                Make sure the QTest API server is running at http://localhost:8080
              </p>
            </div>
          </div>
        </div>
      </main>
    </div>
  );
}
