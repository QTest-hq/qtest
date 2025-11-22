// Package nats provides stream configuration for QTest job processing
package nats

import (
	"context"
	"time"
)

// Stream names
const (
	StreamJobs = "QTEST_JOBS"
)

// Subject patterns for job routing
const (
	// SubjectJobsAll matches all job subjects
	SubjectJobsAll = "jobs.>"

	// Job type subjects
	SubjectJobIngestion   = "jobs.ingestion"
	SubjectJobModeling    = "jobs.modeling"
	SubjectJobPlanning    = "jobs.planning"
	SubjectJobGeneration  = "jobs.generation"
	SubjectJobValidation  = "jobs.validation"
	SubjectJobMutation    = "jobs.mutation"
	SubjectJobIntegration = "jobs.integration"
)

// Consumer names
const (
	ConsumerIngestion   = "ingestion-worker"
	ConsumerModeling    = "modeling-worker"
	ConsumerPlanning    = "planning-worker"
	ConsumerGeneration  = "generation-worker"
	ConsumerValidation  = "validation-worker"
	ConsumerMutation    = "mutation-worker"
	ConsumerIntegration = "integration-worker"
)

// DefaultStreamConfig returns the default stream configuration for jobs
func DefaultStreamConfig() StreamConfig {
	return StreamConfig{
		Name:        StreamJobs,
		Subjects:    []string{SubjectJobsAll},
		MaxMsgs:     100000,
		MaxBytes:    1024 * 1024 * 500, // 500MB
		MaxAge:      7 * 24 * time.Hour,
		Replicas:    1,
		Description: "QTest job processing stream",
	}
}

// SetupStreams creates all required streams and consumers
func (c *Client) SetupStreams(ctx context.Context) error {
	// Create main jobs stream
	_, err := c.CreateStream(ctx, DefaultStreamConfig())
	if err != nil {
		return err
	}

	// Create consumers for each worker type
	consumers := []struct {
		name    string
		subject string
	}{
		{ConsumerIngestion, SubjectJobIngestion},
		{ConsumerModeling, SubjectJobModeling},
		{ConsumerPlanning, SubjectJobPlanning},
		{ConsumerGeneration, SubjectJobGeneration},
		{ConsumerValidation, SubjectJobValidation},
		{ConsumerMutation, SubjectJobMutation},
		{ConsumerIntegration, SubjectJobIntegration},
	}

	for _, cons := range consumers {
		if _, err := c.CreateConsumer(ctx, StreamJobs, cons.name, cons.subject); err != nil {
			return err
		}
	}

	return nil
}

// SubjectForJobType returns the NATS subject for a job type
func SubjectForJobType(jobType string) string {
	switch jobType {
	case "ingestion":
		return SubjectJobIngestion
	case "modeling":
		return SubjectJobModeling
	case "planning":
		return SubjectJobPlanning
	case "generation":
		return SubjectJobGeneration
	case "validation":
		return SubjectJobValidation
	case "mutation":
		return SubjectJobMutation
	case "integration":
		return SubjectJobIntegration
	default:
		return ""
	}
}

// ConsumerForJobType returns the consumer name for a job type
func ConsumerForJobType(jobType string) string {
	switch jobType {
	case "ingestion":
		return ConsumerIngestion
	case "modeling":
		return ConsumerModeling
	case "planning":
		return ConsumerPlanning
	case "generation":
		return ConsumerGeneration
	case "validation":
		return ConsumerValidation
	case "mutation":
		return ConsumerMutation
	case "integration":
		return ConsumerIntegration
	default:
		return ""
	}
}
