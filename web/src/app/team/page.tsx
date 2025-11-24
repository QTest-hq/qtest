"use client";

import { useState, useEffect, useCallback } from "react";
import { useAuth } from "@/contexts/AuthContext";
import {
  api,
  OrganizationWithRole,
  OrganizationMember,
  Repository,
  Job,
  MemberRole,
} from "@/lib/api";

interface OrgStats {
  totalRepos: number;
  totalJobs: number;
  completedJobs: number;
  failedJobs: number;
  avgCoverage: number;
}

export default function TeamPage() {
  const { user, loading: authLoading } = useAuth();
  const [organizations, setOrganizations] = useState<OrganizationWithRole[]>([]);
  const [selectedOrg, setSelectedOrg] = useState<OrganizationWithRole | null>(null);
  const [members, setMembers] = useState<OrganizationMember[]>([]);
  const [repos, setRepos] = useState<Repository[]>([]);
  const [jobs, setJobs] = useState<Job[]>([]);
  const [stats, setStats] = useState<OrgStats | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // Create org modal
  const [showCreateModal, setShowCreateModal] = useState(false);
  const [newOrgName, setNewOrgName] = useState("");
  const [newOrgSlug, setNewOrgSlug] = useState("");
  const [newOrgDesc, setNewOrgDesc] = useState("");
  const [creating, setCreating] = useState(false);

  // Invite member modal
  const [showInviteModal, setShowInviteModal] = useState(false);
  const [inviteUserId, setInviteUserId] = useState("");
  const [inviteRole, setInviteRole] = useState<MemberRole>("member");
  const [inviting, setInviting] = useState(false);

  const loadOrganizations = useCallback(async () => {
    try {
      const orgs = await api.listOrganizations();
      setOrganizations(orgs);
      if (orgs.length > 0 && !selectedOrg) {
        // Select first non-personal org, or personal if none
        const nonPersonal = orgs.find((o) => !o.is_personal);
        setSelectedOrg(nonPersonal || orgs[0]);
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load organizations");
    }
  }, [selectedOrg]);

  const loadOrgData = useCallback(async (orgId: string) => {
    try {
      setLoading(true);
      const [membersData, reposData, jobsData] = await Promise.all([
        api.listMembers(orgId),
        api.getOrgRepos(orgId).catch(() => []),
        api.getOrgJobs(orgId).catch(() => []),
      ]);

      setMembers(membersData);
      setRepos(reposData);
      setJobs(jobsData);

      // Calculate stats from loaded data
      const completedJobs = jobsData.filter((j) => j.status === "completed").length;
      const failedJobs = jobsData.filter((j) => j.status === "failed").length;

      setStats({
        totalRepos: reposData.length,
        totalJobs: jobsData.length,
        completedJobs,
        failedJobs,
        avgCoverage: 0, // Would need coverage API
      });
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load organization data");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    if (!authLoading && user) {
      loadOrganizations();
    }
  }, [authLoading, user, loadOrganizations]);

  useEffect(() => {
    if (selectedOrg) {
      loadOrgData(selectedOrg.id);
    }
  }, [selectedOrg, loadOrgData]);

  const handleCreateOrg = async () => {
    if (!newOrgName || !newOrgSlug) return;

    try {
      setCreating(true);
      const org = await api.createOrganization({
        name: newOrgName,
        slug: newOrgSlug,
        description: newOrgDesc || undefined,
      });
      setShowCreateModal(false);
      setNewOrgName("");
      setNewOrgSlug("");
      setNewOrgDesc("");
      await loadOrganizations();
      // Select the new org
      setSelectedOrg({ ...org, role: "owner" });
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to create organization");
    } finally {
      setCreating(false);
    }
  };

  const handleInviteMember = async () => {
    if (!selectedOrg || !inviteUserId) return;

    try {
      setInviting(true);
      await api.addMember(selectedOrg.id, {
        user_id: inviteUserId,
        role: inviteRole,
      });
      setShowInviteModal(false);
      setInviteUserId("");
      setInviteRole("member");
      await loadOrgData(selectedOrg.id);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to invite member");
    } finally {
      setInviting(false);
    }
  };

  const handleRemoveMember = async (userId: string) => {
    if (!selectedOrg) return;
    if (!confirm("Are you sure you want to remove this member?")) return;

    try {
      await api.removeMember(selectedOrg.id, userId);
      await loadOrgData(selectedOrg.id);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to remove member");
    }
  };

  const handleUpdateRole = async (userId: string, role: MemberRole) => {
    if (!selectedOrg) return;

    try {
      await api.updateMemberRole(selectedOrg.id, userId, role);
      await loadOrgData(selectedOrg.id);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to update role");
    }
  };

  const canManage = selectedOrg?.role === "owner" || selectedOrg?.role === "admin";

  if (authLoading) {
    return (
      <div className="p-8">
        <div className="animate-pulse">Loading...</div>
      </div>
    );
  }

  if (!user) {
    return (
      <div className="p-8">
        <div className="bg-yellow-50 border border-yellow-200 rounded-lg p-4">
          <p className="text-yellow-800">Please sign in to view team dashboard.</p>
        </div>
      </div>
    );
  }

  return (
    <div className="p-8">
      <div className="max-w-7xl mx-auto">
        {/* Header */}
        <div className="flex items-center justify-between mb-8">
          <div>
            <h1 className="text-2xl font-bold text-gray-900">Team Dashboard</h1>
            <p className="text-gray-600 mt-1">
              Manage your organizations and team members
            </p>
          </div>
          <button
            onClick={() => setShowCreateModal(true)}
            className="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700"
          >
            Create Organization
          </button>
        </div>

        {error && (
          <div className="mb-6 bg-red-50 border border-red-200 rounded-lg p-4">
            <p className="text-red-800">{error}</p>
            <button
              onClick={() => setError(null)}
              className="text-red-600 underline text-sm mt-1"
            >
              Dismiss
            </button>
          </div>
        )}

        <div className="grid grid-cols-12 gap-6">
          {/* Sidebar - Organization List */}
          <div className="col-span-3">
            <div className="bg-white rounded-lg shadow p-4">
              <h2 className="font-semibold text-gray-900 mb-4">Organizations</h2>
              <div className="space-y-2">
                {organizations.map((org) => (
                  <button
                    key={org.id}
                    onClick={() => setSelectedOrg(org)}
                    className={`w-full text-left px-3 py-2 rounded-lg transition-colors ${
                      selectedOrg?.id === org.id
                        ? "bg-blue-50 text-blue-700 border border-blue-200"
                        : "hover:bg-gray-50 text-gray-700"
                    }`}
                  >
                    <div className="font-medium">{org.name}</div>
                    <div className="text-xs text-gray-500 flex items-center gap-2">
                      <span className="capitalize">{org.role}</span>
                      {org.is_personal && (
                        <span className="bg-gray-100 px-1.5 py-0.5 rounded">
                          Personal
                        </span>
                      )}
                    </div>
                  </button>
                ))}
                {organizations.length === 0 && (
                  <p className="text-gray-500 text-sm py-4 text-center">
                    No organizations yet
                  </p>
                )}
              </div>
            </div>
          </div>

          {/* Main Content */}
          <div className="col-span-9">
            {selectedOrg ? (
              <div className="space-y-6">
                {/* Org Header */}
                <div className="bg-white rounded-lg shadow p-6">
                  <div className="flex items-start justify-between">
                    <div>
                      <h2 className="text-xl font-bold text-gray-900">
                        {selectedOrg.name}
                      </h2>
                      <p className="text-gray-500 text-sm">@{selectedOrg.slug}</p>
                      {selectedOrg.description && (
                        <p className="text-gray-600 mt-2">{selectedOrg.description}</p>
                      )}
                    </div>
                    <span
                      className={`px-3 py-1 rounded-full text-sm font-medium ${
                        selectedOrg.role === "owner"
                          ? "bg-purple-100 text-purple-700"
                          : selectedOrg.role === "admin"
                          ? "bg-blue-100 text-blue-700"
                          : "bg-gray-100 text-gray-700"
                      }`}
                    >
                      {selectedOrg.role}
                    </span>
                  </div>
                </div>

                {/* Stats Grid */}
                {loading ? (
                  <div className="grid grid-cols-4 gap-4">
                    {[...Array(4)].map((_, i) => (
                      <div
                        key={i}
                        className="bg-white rounded-lg shadow p-4 animate-pulse"
                      >
                        <div className="h-4 bg-gray-200 rounded w-20 mb-2" />
                        <div className="h-8 bg-gray-200 rounded w-16" />
                      </div>
                    ))}
                  </div>
                ) : (
                  <div className="grid grid-cols-4 gap-4">
                    <div className="bg-white rounded-lg shadow p-4">
                      <div className="text-sm text-gray-500">Repositories</div>
                      <div className="text-2xl font-bold text-gray-900">
                        {stats?.totalRepos || 0}
                      </div>
                    </div>
                    <div className="bg-white rounded-lg shadow p-4">
                      <div className="text-sm text-gray-500">Total Jobs</div>
                      <div className="text-2xl font-bold text-gray-900">
                        {stats?.totalJobs || 0}
                      </div>
                    </div>
                    <div className="bg-white rounded-lg shadow p-4">
                      <div className="text-sm text-gray-500">Completed</div>
                      <div className="text-2xl font-bold text-green-600">
                        {stats?.completedJobs || 0}
                      </div>
                    </div>
                    <div className="bg-white rounded-lg shadow p-4">
                      <div className="text-sm text-gray-500">Failed</div>
                      <div className="text-2xl font-bold text-red-600">
                        {stats?.failedJobs || 0}
                      </div>
                    </div>
                  </div>
                )}

                {/* Members Section */}
                <div className="bg-white rounded-lg shadow">
                  <div className="p-4 border-b flex items-center justify-between">
                    <h3 className="font-semibold text-gray-900">
                      Team Members ({members.length})
                    </h3>
                    {canManage && (
                      <button
                        onClick={() => setShowInviteModal(true)}
                        className="px-3 py-1.5 bg-blue-600 text-white text-sm rounded-lg hover:bg-blue-700"
                      >
                        Invite Member
                      </button>
                    )}
                  </div>
                  <div className="divide-y">
                    {members.map((member) => (
                      <div
                        key={member.id}
                        className="p-4 flex items-center justify-between"
                      >
                        <div className="flex items-center gap-3">
                          {member.avatar_url ? (
                            <img
                              src={member.avatar_url}
                              alt={member.github_login}
                              className="w-10 h-10 rounded-full"
                            />
                          ) : (
                            <div className="w-10 h-10 rounded-full bg-gray-200 flex items-center justify-center">
                              <span className="text-gray-500 font-medium">
                                {member.github_login.charAt(0).toUpperCase()}
                              </span>
                            </div>
                          )}
                          <div>
                            <div className="font-medium text-gray-900">
                              {member.name || member.github_login}
                            </div>
                            <div className="text-sm text-gray-500">
                              @{member.github_login}
                            </div>
                          </div>
                        </div>
                        <div className="flex items-center gap-3">
                          {canManage && member.role !== "owner" ? (
                            <select
                              value={member.role}
                              onChange={(e) =>
                                handleUpdateRole(
                                  member.user_id,
                                  e.target.value as MemberRole
                                )
                              }
                              className="border rounded px-2 py-1 text-sm"
                            >
                              <option value="admin">Admin</option>
                              <option value="member">Member</option>
                              <option value="viewer">Viewer</option>
                            </select>
                          ) : (
                            <span
                              className={`px-2 py-1 rounded text-sm ${
                                member.role === "owner"
                                  ? "bg-purple-100 text-purple-700"
                                  : member.role === "admin"
                                  ? "bg-blue-100 text-blue-700"
                                  : "bg-gray-100 text-gray-700"
                              }`}
                            >
                              {member.role}
                            </span>
                          )}
                          {canManage && member.role !== "owner" && (
                            <button
                              onClick={() => handleRemoveMember(member.user_id)}
                              className="text-red-600 hover:text-red-700 text-sm"
                            >
                              Remove
                            </button>
                          )}
                        </div>
                      </div>
                    ))}
                    {members.length === 0 && (
                      <div className="p-8 text-center text-gray-500">
                        No members yet
                      </div>
                    )}
                  </div>
                </div>

                {/* Recent Repos */}
                <div className="bg-white rounded-lg shadow">
                  <div className="p-4 border-b">
                    <h3 className="font-semibold text-gray-900">
                      Recent Repositories ({repos.length})
                    </h3>
                  </div>
                  <div className="divide-y">
                    {repos.slice(0, 5).map((repo) => (
                      <div key={repo.id} className="p-4 flex items-center justify-between">
                        <div>
                          <div className="font-medium text-gray-900">
                            {repo.owner}/{repo.name}
                          </div>
                          <div className="text-sm text-gray-500">
                            {repo.default_branch} &middot; {repo.status}
                          </div>
                        </div>
                        <a
                          href={`/repos/${repo.id}`}
                          className="text-blue-600 hover:text-blue-700 text-sm"
                        >
                          View
                        </a>
                      </div>
                    ))}
                    {repos.length === 0 && (
                      <div className="p-8 text-center text-gray-500">
                        No repositories yet
                      </div>
                    )}
                  </div>
                </div>

                {/* Recent Jobs */}
                <div className="bg-white rounded-lg shadow">
                  <div className="p-4 border-b">
                    <h3 className="font-semibold text-gray-900">
                      Recent Jobs ({jobs.length})
                    </h3>
                  </div>
                  <div className="divide-y">
                    {jobs.slice(0, 5).map((job) => (
                      <div key={job.id} className="p-4 flex items-center justify-between">
                        <div>
                          <div className="font-medium text-gray-900">
                            {job.type}
                          </div>
                          <div className="text-sm text-gray-500">
                            {new Date(job.created_at).toLocaleString()}
                          </div>
                        </div>
                        <span
                          className={`px-2 py-1 rounded text-sm ${
                            job.status === "completed"
                              ? "bg-green-100 text-green-700"
                              : job.status === "failed"
                              ? "bg-red-100 text-red-700"
                              : job.status === "running"
                              ? "bg-blue-100 text-blue-700"
                              : "bg-gray-100 text-gray-700"
                          }`}
                        >
                          {job.status}
                        </span>
                      </div>
                    ))}
                    {jobs.length === 0 && (
                      <div className="p-8 text-center text-gray-500">
                        No jobs yet
                      </div>
                    )}
                  </div>
                </div>
              </div>
            ) : (
              <div className="bg-white rounded-lg shadow p-8 text-center">
                <p className="text-gray-500">
                  Select an organization or create a new one
                </p>
              </div>
            )}
          </div>
        </div>
      </div>

      {/* Create Organization Modal */}
      {showCreateModal && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <div className="bg-white rounded-lg shadow-xl w-full max-w-md p-6">
            <h2 className="text-xl font-bold text-gray-900 mb-4">
              Create Organization
            </h2>
            <div className="space-y-4">
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">
                  Name
                </label>
                <input
                  type="text"
                  value={newOrgName}
                  onChange={(e) => setNewOrgName(e.target.value)}
                  placeholder="My Team"
                  className="w-full border rounded-lg px-3 py-2"
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">
                  Slug
                </label>
                <input
                  type="text"
                  value={newOrgSlug}
                  onChange={(e) =>
                    setNewOrgSlug(e.target.value.toLowerCase().replace(/[^a-z0-9-]/g, ""))
                  }
                  placeholder="my-team"
                  className="w-full border rounded-lg px-3 py-2"
                />
                <p className="text-xs text-gray-500 mt-1">
                  Used in URLs: /team/{newOrgSlug || "my-team"}
                </p>
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">
                  Description (optional)
                </label>
                <textarea
                  value={newOrgDesc}
                  onChange={(e) => setNewOrgDesc(e.target.value)}
                  placeholder="A brief description of your organization"
                  rows={3}
                  className="w-full border rounded-lg px-3 py-2"
                />
              </div>
            </div>
            <div className="flex justify-end gap-3 mt-6">
              <button
                onClick={() => setShowCreateModal(false)}
                className="px-4 py-2 border rounded-lg hover:bg-gray-50"
              >
                Cancel
              </button>
              <button
                onClick={handleCreateOrg}
                disabled={!newOrgName || !newOrgSlug || creating}
                className="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 disabled:opacity-50"
              >
                {creating ? "Creating..." : "Create"}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Invite Member Modal */}
      {showInviteModal && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <div className="bg-white rounded-lg shadow-xl w-full max-w-md p-6">
            <h2 className="text-xl font-bold text-gray-900 mb-4">
              Invite Team Member
            </h2>
            <div className="space-y-4">
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">
                  User ID
                </label>
                <input
                  type="text"
                  value={inviteUserId}
                  onChange={(e) => setInviteUserId(e.target.value)}
                  placeholder="User UUID"
                  className="w-full border rounded-lg px-3 py-2"
                />
                <p className="text-xs text-gray-500 mt-1">
                  Enter the UUID of the user to invite
                </p>
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">
                  Role
                </label>
                <select
                  value={inviteRole}
                  onChange={(e) => setInviteRole(e.target.value as MemberRole)}
                  className="w-full border rounded-lg px-3 py-2"
                >
                  <option value="admin">Admin - Can manage members and settings</option>
                  <option value="member">Member - Can create and run tests</option>
                  <option value="viewer">Viewer - Read-only access</option>
                </select>
              </div>
            </div>
            <div className="flex justify-end gap-3 mt-6">
              <button
                onClick={() => setShowInviteModal(false)}
                className="px-4 py-2 border rounded-lg hover:bg-gray-50"
              >
                Cancel
              </button>
              <button
                onClick={handleInviteMember}
                disabled={!inviteUserId || inviting}
                className="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 disabled:opacity-50"
              >
                {inviting ? "Inviting..." : "Invite"}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
