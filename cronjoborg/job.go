package cronjoborg

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// listJobsResponse is the raw shape of GET /jobs.
type listJobsResponse struct {
	Jobs       []Job `json:"jobs"`
	SomeFailed bool  `json:"someFailed"`
}

// jobDetailsResponse is the raw shape of GET /jobs/{jobId}.
type jobDetailsResponse struct {
	JobDetails DetailedJob `json:"jobDetails"`
}

// createJobResponse is the raw shape of PUT /jobs.
type createJobResponse struct {
	JobID int64 `json:"jobId"`
}

// jobEnvelope wraps a JobInput the way both PUT /jobs and PATCH /jobs/{jobId}
// expect it: {"job": {…}}.
type jobEnvelope struct {
	Job JobInput `json:"job"`
}

// ListJobs returns every job in the account.
//
// someFailed is true when the API could not retrieve some jobs because of
// internal errors, which makes the returned slice INCOMPLETE. Reconciliation
// logic must treat that as "unknown", not as "those jobs are gone" — deleting
// or recreating on a partial list is how a sweep destroys jobs it did not mean
// to touch.
//
// Rate limit: 5 requests per second.
func (c *Client) ListJobs(ctx context.Context) (jobs []Job, someFailed bool, err error) {
	var response listJobsResponse
	if err := c.doJSON(ctx, http.MethodGet, pathJobs, nil, &response); err != nil {
		return nil, false, err
	}
	return response.Jobs, response.SomeFailed, nil
}

// GetJob returns the full settings of one job, including the auth, notification
// and extended-data sections that ListJobs omits.
//
// Rate limit: 5 requests per second.
func (c *Client) GetJob(ctx context.Context, jobID int64) (*DetailedJob, error) {
	if jobID <= 0 {
		return nil, errors.New("cronjoborg: jobID is required")
	}

	var response jobDetailsResponse
	if err := c.doJSON(ctx, http.MethodGet, jobPath(jobID), nil, &response); err != nil {
		return nil, err
	}
	return &response.JobDetails, nil
}

// CreateJob creates a job and returns its identifier. Only input.URL is
// mandatory; every omitted field takes the API's documented default, which
// notably means the job is created DISABLED (Enabled defaults to false) and
// with an empty schedule, so it never runs until both are set.
//
// Rate limit: 1 request per second AND 5 requests per minute — the strictest
// method on the API. Creating jobs in a loop needs pacing.
func (c *Client) CreateJob(ctx context.Context, input JobInput) (int64, error) {
	if input.URL == nil {
		return 0, errors.New("cronjoborg: JobInput.URL is required to create a job")
	}
	if err := validateJobURL(*input.URL); err != nil {
		return 0, err
	}

	var response createJobResponse
	if err := c.doJSON(ctx, http.MethodPut, pathJobs, jobEnvelope{Job: input}, &response); err != nil {
		return 0, err
	}
	return response.JobID, nil
}

// UpdateJob applies a delta to a job: the fields set on input are changed, the
// rest are left as they are. An input with no field set sends an empty delta
// and changes nothing, which is why it is rejected here rather than spent
// against the request quota.
//
// Rate limit: 5 requests per second.
func (c *Client) UpdateJob(ctx context.Context, jobID int64, input JobInput) error {
	if jobID <= 0 {
		return errors.New("cronjoborg: jobID is required")
	}
	if input.isEmpty() {
		return errors.New("cronjoborg: UpdateJob delta is empty — set at least one field")
	}
	if input.URL != nil {
		if err := validateJobURL(*input.URL); err != nil {
			return err
		}
	}

	return c.doJSON(ctx, http.MethodPatch, jobPath(jobID), jobEnvelope{Job: input}, nil)
}

// SetJobEnabled enables or disables a job. It is the delta every caller writes
// by hand otherwise, and pausing beats deleting: the job keeps its id, its
// schedule and its history.
//
// Rate limit: 5 requests per second.
func (c *Client) SetJobEnabled(ctx context.Context, jobID int64, enabled bool) error {
	return c.UpdateJob(ctx, jobID, JobInput{Enabled: &enabled})
}

// DeleteJob removes a job permanently, together with its execution history.
//
// Rate limit: 5 requests per second.
func (c *Client) DeleteJob(ctx context.Context, jobID int64) error {
	if jobID <= 0 {
		return errors.New("cronjoborg: jobID is required")
	}
	return c.doJSON(ctx, http.MethodDelete, jobPath(jobID), nil, nil)
}

// isEmpty reports whether the input carries no field at all, i.e. whether it
// would serialize to an empty delta.
func (i JobInput) isEmpty() bool {
	return i.Enabled == nil &&
		i.Title == nil &&
		i.SaveResponses == nil &&
		i.URL == nil &&
		i.RequestTimeout == nil &&
		i.RedirectSuccess == nil &&
		i.FolderID == nil &&
		i.Schedule == nil &&
		i.RequestMethod == nil &&
		i.Auth == nil &&
		i.Notification == nil &&
		i.ExtendedData == nil
}

// validateJobURL rejects a URL the scheduler could never call, so the mistake
// surfaces locally instead of as a 400 that spends the daily request quota.
func validateJobURL(rawURL string) error {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return errors.New("cronjoborg: job URL is empty")
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return fmt.Errorf("cronjoborg: invalid job URL: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("cronjoborg: job URL must be http or https, got %q", parsed.Scheme)
	}
	if parsed.Host == "" {
		return errors.New("cronjoborg: job URL has no host")
	}
	return nil
}

// jobPath builds /jobs/{jobId}.
func jobPath(jobID int64) string {
	return pathJobs + "/" + strconv.FormatInt(jobID, 10)
}
