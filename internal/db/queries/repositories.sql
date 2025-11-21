-- name: CreateRepository :one
INSERT INTO repositories (url, name, owner, default_branch, language)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetRepository :one
SELECT * FROM repositories WHERE id = $1;

-- name: GetRepositoryByURL :one
SELECT * FROM repositories WHERE url = $1;

-- name: ListRepositories :many
SELECT * FROM repositories
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: UpdateRepositoryStatus :one
UPDATE repositories
SET status = $2, last_commit_sha = $3
WHERE id = $1
RETURNING *;

-- name: DeleteRepository :exec
DELETE FROM repositories WHERE id = $1;

-- name: CreateSystemModel :one
INSERT INTO system_models (repository_id, commit_sha, model_data)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetSystemModel :one
SELECT * FROM system_models WHERE id = $1;

-- name: GetLatestSystemModel :one
SELECT * FROM system_models
WHERE repository_id = $1
ORDER BY created_at DESC
LIMIT 1;

-- name: CreateGenerationRun :one
INSERT INTO generation_runs (repository_id, system_model_id, config)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetGenerationRun :one
SELECT * FROM generation_runs WHERE id = $1;

-- name: ListGenerationRuns :many
SELECT * FROM generation_runs
WHERE repository_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: UpdateGenerationRunStatus :one
UPDATE generation_runs
SET status = $2,
    started_at = CASE WHEN $2 = 'running' THEN CURRENT_TIMESTAMP ELSE started_at END,
    completed_at = CASE WHEN $2 IN ('completed', 'failed') THEN CURRENT_TIMESTAMP ELSE completed_at END
WHERE id = $1
RETURNING *;

-- name: UpdateGenerationRunSummary :one
UPDATE generation_runs
SET summary = $2
WHERE id = $1
RETURNING *;

-- name: CreateGeneratedTest :one
INSERT INTO generated_tests (run_id, name, type, target_file, target_function, dsl, framework)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetGeneratedTest :one
SELECT * FROM generated_tests WHERE id = $1;

-- name: ListGeneratedTestsByRun :many
SELECT * FROM generated_tests
WHERE run_id = $1
ORDER BY created_at;

-- name: UpdateGeneratedTestStatus :one
UPDATE generated_tests
SET status = $2, rejection_reason = $3
WHERE id = $1
RETURNING *;

-- name: UpdateGeneratedTestCode :one
UPDATE generated_tests
SET generated_code = $2
WHERE id = $1
RETURNING *;

-- name: UpdateGeneratedTestMutationScore :one
UPDATE generated_tests
SET mutation_score = $2
WHERE id = $1
RETURNING *;

-- name: CreateMutationResult :one
INSERT INTO mutation_results (test_id, mutant_id, operator, location, killed, runtime_ms)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: ListMutationResultsByTest :many
SELECT * FROM mutation_results
WHERE test_id = $1
ORDER BY created_at;

-- name: GetMutationScore :one
SELECT
    COUNT(*) FILTER (WHERE killed = true)::float / NULLIF(COUNT(*)::float, 0) as score
FROM mutation_results
WHERE test_id = $1;
