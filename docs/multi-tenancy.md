# Multi-Tenancy & API Key Documentation

QTest supports multi-tenant architecture with organization-based data isolation, role-based access control (RBAC), API key authentication, and comprehensive audit logging.

## Table of Contents

- [Architecture Overview](#architecture-overview)
- [Authentication](#authentication)
- [Organizations & Members](#organizations--members)
- [API Keys](#api-keys)
- [Authorization & Scopes](#authorization--scopes)
- [Audit Logging](#audit-logging)
- [API Reference](#api-reference)

---

## Architecture Overview

### Data Model

```
┌─────────────┐     ┌─────────────────────┐     ┌──────────────┐
│   Users     │────▶│ Organization Members│◀────│ Organizations│
└─────────────┘     └─────────────────────┘     └──────────────┘
      │                                                │
      │                                                │
      ▼                                                ▼
┌─────────────┐                                 ┌──────────────┐
│  Sessions   │                                 │ Repositories │
└─────────────┘                                 └──────────────┘
      │                                                │
      ▼                                                ▼
┌─────────────┐                                 ┌──────────────┐
│  API Keys   │                                 │ Runs/Tests   │
└─────────────┘                                 └──────────────┘
```

### Key Concepts

- **Users**: Authenticated via GitHub OAuth, identified by GitHub ID
- **Organizations**: Tenant containers that own repositories and resources
- **Personal Organizations**: Auto-created for each user (cannot be deleted)
- **Members**: Users belong to organizations with specific roles
- **API Keys**: Programmatic access with scoped permissions

---

## Authentication

### GitHub OAuth Flow

1. User visits `/auth/login`
2. Redirected to GitHub for authorization
3. GitHub redirects to `/auth/callback` with code
4. Server exchanges code for access token
5. User info fetched from GitHub API
6. Session created and cookie set

### Session-Based Auth

Sessions are stored in-memory with configurable TTL (default: 24 hours).

**Headers:**
```
Cookie: qtest_session=<session_id>
# or
Authorization: Bearer <session_id>
```

### API Key Auth

API keys provide programmatic access with scoped permissions.

**Headers:**
```
Authorization: Bearer qtest_<key>
# or
X-API-Key: qtest_<key>
```

---

## Organizations & Members

### Organization Types

| Type | Description |
|------|-------------|
| `personal` | Auto-created for each user, cannot be deleted |
| `team` | Created by users for team collaboration |

### Member Roles

| Role | Permissions |
|------|-------------|
| `owner` | Full access, can delete org, manage all members |
| `admin` | Manage members, create API keys, view audit logs |
| `member` | Create/manage repositories and runs |
| `viewer` | Read-only access to repositories and runs |

### Creating Organizations

```bash
curl -X POST http://localhost:8080/api/v1/organizations \
  -H "Authorization: Bearer <session>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "My Team",
    "display_name": "My Team Organization"
  }'
```

### Managing Members

```bash
# Add member
curl -X POST http://localhost:8080/api/v1/organizations/{org_id}/members \
  -H "Authorization: Bearer <session>" \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "<user_uuid>",
    "role": "member"
  }'

# Update role
curl -X PATCH http://localhost:8080/api/v1/organizations/{org_id}/members/{user_id} \
  -H "Authorization: Bearer <session>" \
  -H "Content-Type: application/json" \
  -d '{"role": "admin"}'

# Remove member
curl -X DELETE http://localhost:8080/api/v1/organizations/{org_id}/members/{user_id} \
  -H "Authorization: Bearer <session>"
```

---

## API Keys

### Creating API Keys

```bash
curl -X POST http://localhost:8080/api/v1/api-keys \
  -H "Authorization: Bearer <session>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "CI/CD Pipeline",
    "organization_id": "<org_uuid>",  // optional, defaults to personal org
    "scopes": ["repos:read", "runs:write", "jobs:write"],
    "expires_in_days": 90  // optional
  }'
```

**Response:**
```json
{
  "id": "uuid",
  "organization_id": "uuid",
  "name": "CI/CD Pipeline",
  "key_prefix": "qtest_a1b2c3d4",
  "scopes": ["repos:read", "runs:write", "jobs:write"],
  "expires_at": "2024-03-15T00:00:00Z",
  "created_at": "2023-12-15T10:30:00Z",
  "secret": "qtest_a1b2c3d4e5f6g7h8..."  // Only returned on creation!
}
```

**Important:** The `secret` is only returned once at creation time. Store it securely!

### Key Format

API keys follow the format: `qtest_<64_hex_chars>`

- Prefix: `qtest_`
- Key prefix (for display): first 8 chars after prefix
- Full key: 70 characters total
- Storage: SHA256 hash (key is never stored)

### Listing API Keys

```bash
# List user's API keys
curl http://localhost:8080/api/v1/api-keys \
  -H "Authorization: Bearer <session>"

# List org's API keys
curl "http://localhost:8080/api/v1/api-keys?organization_id=<org_uuid>" \
  -H "Authorization: Bearer <session>"
```

### Revoking API Keys

```bash
curl -X DELETE http://localhost:8080/api/v1/api-keys/{key_id} \
  -H "Authorization: Bearer <session>"
```

---

## Authorization & Scopes

### Available Scopes

| Scope | Description |
|-------|-------------|
| `repos:read` | List and view repositories |
| `repos:write` | Create and delete repositories |
| `runs:read` | List and view generation runs |
| `runs:write` | Create generation runs |
| `tests:read` | List and view generated tests |
| `tests:write` | Accept or reject tests |
| `jobs:read` | List and view jobs |
| `jobs:write` | Create, cancel, and retry jobs |
| `mutation:read` | List and view mutation testing results |
| `admin` | Full access to all resources |

### Route-to-Scope Mapping

| Endpoint | Method | Required Scope |
|----------|--------|----------------|
| `/api/v1/repos` | GET | `repos:read` |
| `/api/v1/repos` | POST | `repos:write` |
| `/api/v1/repos/{id}` | GET | `repos:read` |
| `/api/v1/repos/{id}` | DELETE | `repos:write` |
| `/api/v1/repos/{id}/jobs` | GET | `jobs:read` |
| `/api/v1/repos/{id}/runs` | GET | `runs:read` |
| `/api/v1/repos/{id}/runs` | POST | `runs:write` |
| `/api/v1/repos/{id}/runs/{id}` | GET | `runs:read` |
| `/api/v1/repos/{id}/runs/{id}/tests` | GET | `tests:read` |
| `/api/v1/tests` | GET | `tests:read` |
| `/api/v1/tests/{id}` | GET | `tests:read` |
| `/api/v1/tests/{id}/accept` | PUT | `tests:write` |
| `/api/v1/tests/{id}/reject` | PUT | `tests:write` |
| `/api/v1/jobs` | GET | `jobs:read` |
| `/api/v1/jobs` | POST | `jobs:write` |
| `/api/v1/jobs/pipeline` | POST | `jobs:write` |
| `/api/v1/jobs/{id}` | GET | `jobs:read` |
| `/api/v1/jobs/{id}/cancel` | POST | `jobs:write` |
| `/api/v1/jobs/{id}/retry` | POST | `jobs:write` |
| `/api/v1/mutation` | GET | `mutation:read` |
| `/api/v1/mutation` | POST | `mutation:read` |
| `/api/v1/mutation/{id}` | GET | `mutation:read` |
| `/api/v1/organizations/*` | * | `admin` (session only) |
| `/api/v1/api-keys/*` | * | `admin` (session only) |

### Scope Behavior

- **Session auth**: Full access to all routes (scopes don't apply)
- **API key auth**: Must have required scope or receive `403 Forbidden`
- **`admin` scope**: Grants access to all routes

---

## Audit Logging

All sensitive operations are logged for compliance and security.

### Logged Actions

| Action | Description |
|--------|-------------|
| `login` | User login via GitHub OAuth |
| `logout` | User logout |
| `org.create` | Organization created |
| `org.update` | Organization updated |
| `org.delete` | Organization deleted |
| `member.add` | Member added to organization |
| `member.remove` | Member removed from organization |
| `member.role_change` | Member role changed |
| `repo.create` | Repository created |
| `repo.delete` | Repository deleted |
| `api_key.create` | API key created |
| `api_key.revoke` | API key revoked |
| `run.create` | Generation run created |
| `test.accept` | Test accepted |
| `test.reject` | Test rejected |

### Viewing Audit Logs

```bash
# Organization audit logs (admin only)
curl "http://localhost:8080/api/v1/organizations/{org_id}/audit-logs?limit=50&offset=0" \
  -H "Authorization: Bearer <session>"

# User's audit logs
curl "http://localhost:8080/api/v1/me/audit-logs?limit=50&offset=0" \
  -H "Authorization: Bearer <session>"
```

**Response:**
```json
[
  {
    "id": "uuid",
    "organization_id": "uuid",
    "user_id": "uuid",
    "action": "api_key.create",
    "resource_type": "api_key",
    "resource_id": "uuid",
    "details": {"name": "CI Pipeline"},
    "ip_address": "192.168.1.1",
    "user_agent": "curl/7.68.0",
    "created_at": "2023-12-15T10:30:00Z"
  }
]
```

---

## API Reference

### Authentication Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/auth/login` | Initiate GitHub OAuth flow |
| GET | `/auth/callback` | GitHub OAuth callback |
| POST | `/auth/logout` | End session |
| GET | `/api/v1/auth/me` | Get current user info |

### Organization Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/organizations` | List user's organizations |
| POST | `/api/v1/organizations` | Create organization |
| GET | `/api/v1/organizations/{id}` | Get organization |
| PATCH | `/api/v1/organizations/{id}` | Update organization |
| DELETE | `/api/v1/organizations/{id}` | Delete organization |
| GET | `/api/v1/organizations/{id}/members` | List members |
| POST | `/api/v1/organizations/{id}/members` | Add member |
| PATCH | `/api/v1/organizations/{id}/members/{user_id}` | Update member role |
| DELETE | `/api/v1/organizations/{id}/members/{user_id}` | Remove member |
| GET | `/api/v1/organizations/{id}/audit-logs` | List audit logs |

### API Key Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/api-keys` | List API keys |
| POST | `/api/v1/api-keys` | Create API key |
| GET | `/api/v1/api-keys/{id}` | Get API key details |
| DELETE | `/api/v1/api-keys/{id}` | Revoke API key |

### User Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/me/audit-logs` | List user's audit logs |

---

## Database Schema

### Tables

```sql
-- Users
CREATE TABLE users (
    id UUID PRIMARY KEY,
    github_id BIGINT UNIQUE NOT NULL,
    username VARCHAR(255) NOT NULL,
    email VARCHAR(255),
    avatar_url TEXT,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Organizations
CREATE TABLE organizations (
    id UUID PRIMARY KEY,
    name VARCHAR(255) UNIQUE NOT NULL,
    display_name VARCHAR(255),
    type VARCHAR(50) DEFAULT 'team',
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Organization Members
CREATE TABLE organization_members (
    organization_id UUID REFERENCES organizations(id),
    user_id UUID REFERENCES users(id),
    role VARCHAR(50) NOT NULL,
    created_at TIMESTAMP DEFAULT NOW(),
    PRIMARY KEY (organization_id, user_id)
);

-- Sessions
CREATE TABLE sessions (
    id VARCHAR(255) PRIMARY KEY,
    user_id UUID REFERENCES users(id),
    expires_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP DEFAULT NOW()
);

-- API Keys
CREATE TABLE api_keys (
    id UUID PRIMARY KEY,
    organization_id UUID REFERENCES organizations(id),
    user_id UUID REFERENCES users(id),
    name VARCHAR(255) NOT NULL,
    key_prefix VARCHAR(20) NOT NULL,
    key_hash VARCHAR(64) NOT NULL,
    scopes TEXT[] NOT NULL,
    expires_at TIMESTAMP,
    last_used_at TIMESTAMP,
    revoked_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT NOW()
);

-- Audit Logs
CREATE TABLE audit_logs (
    id UUID PRIMARY KEY,
    organization_id UUID REFERENCES organizations(id),
    user_id UUID REFERENCES users(id),
    action VARCHAR(100) NOT NULL,
    resource_type VARCHAR(100) NOT NULL,
    resource_id UUID,
    details JSONB,
    ip_address INET,
    user_agent TEXT,
    created_at TIMESTAMP DEFAULT NOW()
);
```

---

## Security Best Practices

1. **API Key Storage**: Never log or expose full API keys. Store only the hash.
2. **Key Rotation**: Set expiration dates and rotate keys regularly.
3. **Least Privilege**: Grant only the scopes needed for each use case.
4. **Audit Review**: Regularly review audit logs for suspicious activity.
5. **Session Management**: Sessions expire after 24 hours by default.

---

## Migration

To apply the multi-tenancy schema:

```bash
docker exec -i qtest-postgres psql -U qtest -d qtest < migrations/005_multi_tenancy.sql
```
