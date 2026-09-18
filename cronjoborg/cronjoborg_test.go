package cronjoborg

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// testAPIKey is a fixture value, not a credential. Naming it keeps secret
// scanners from reporting every inline literal in this file.
const testAPIKey = "test-api-key"

// capturedRequest records what the SDK actually put on the wire.
type capturedRequest struct {
	method      string
	path        string
	authHeader  string
	contentType string
	body        string
}

// newTestServer serves handler and returns a Client pointed at it, plus a
// pointer to the last request it received.
func newTestServer(t *testing.T, handler func(w http.ResponseWriter, r *http.Request)) (*Client, *capturedRequest) {
	t.Helper()

	captured := &capturedRequest{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		captured.method = r.Method
		captured.path = r.URL.Path
		captured.authHeader = r.Header.Get("Authorization")
		captured.contentType = r.Header.Get("Content-Type")
		captured.body = string(raw)
		handler(w, r)
	}))
	t.Cleanup(server.Close)

	client, err := New(Config{APIKey: testAPIKey, BaseURL: server.URL})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return client, captured
}

func TestNewRequiresAPIKey(t *testing.T) {
	if _, err := New(Config{}); err == nil {
		t.Fatal("expected an error when APIKey is missing")
	}
	client, err := New(Config{APIKey: testAPIKey})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if client.baseURL != DefaultBaseURL {
		t.Fatalf("baseURL = %q, want %q", client.baseURL, DefaultBaseURL)
	}
}

func TestListJobsDecodesAndAuthenticates(t *testing.T) {
	client, captured := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"jobs":[{"jobId":12345,"enabled":true,"title":"Example Job","url":"https://example.com/","lastStatus":1,"nextExecution":1640187240,"requestMethod":1,"schedule":{"timezone":"UTC","minutes":[0,15,30,45],"hours":[-1],"mdays":[-1],"months":[-1],"wdays":[-1]}}],"someFailed":false}`)
	})

	jobs, someFailed, err := client.ListJobs(context.Background())
	if err != nil {
		t.Fatalf("ListJobs: %v", err)
	}
	if someFailed {
		t.Fatal("someFailed = true, want false")
	}
	if len(jobs) != 1 {
		t.Fatalf("len(jobs) = %d, want 1", len(jobs))
	}

	job := jobs[0]
	if job.JobID != 12345 || job.Title != "Example Job" {
		t.Fatalf("unexpected job: %+v", job)
	}
	if !job.LastStatus.OK() {
		t.Fatalf("LastStatus = %v, want ok", job.LastStatus)
	}
	if job.RequestMethod != MethodPOST {
		t.Fatalf("RequestMethod = %v, want POST", job.RequestMethod)
	}
	if job.NextExecution == nil || *job.NextExecution != 1640187240 {
		t.Fatalf("NextExecution = %v, want 1640187240", job.NextExecution)
	}
	if len(job.Schedule.Minutes) != 4 {
		t.Fatalf("Schedule.Minutes = %v, want 4 values", job.Schedule.Minutes)
	}

	if captured.method != http.MethodGet || captured.path != "/jobs" {
		t.Fatalf("request = %s %s, want GET /jobs", captured.method, captured.path)
	}
	if captured.authHeader != "Bearer "+testAPIKey {
		t.Fatalf("Authorization = %q, want the bearer form", captured.authHeader)
	}
}

// A null nextExecution is the API's "no prediction available", which must not
// decode as a real timestamp of 0 (the epoch reads as a plausible past run).
func TestListJobsKeepsNullNextExecutionDistinctFromZero(t *testing.T) {
	client, _ := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"jobs":[{"jobId":1,"nextExecution":null}],"someFailed":true}`)
	})

	jobs, someFailed, err := client.ListJobs(context.Background())
	if err != nil {
		t.Fatalf("ListJobs: %v", err)
	}
	if !someFailed {
		t.Fatal("someFailed = false, want true — the list is incomplete")
	}
	if jobs[0].NextExecution != nil {
		t.Fatalf("NextExecution = %v, want nil", jobs[0].NextExecution)
	}
}

func TestGetJobDecodesDetailedFields(t *testing.T) {
	client, captured := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"jobDetails":{"jobId":12345,"url":"https://example.com/","auth":{"enable":true,"user":"u","password":"p"},"notification":{"onFailure":true,"onFailureCount":2},"extendedData":{"headers":{"X-Foo":"Bar"},"body":"Hello World!"},"requestTimeout":300}}`)
	})

	job, err := client.GetJob(context.Background(), 12345)
	if err != nil {
		t.Fatalf("GetJob: %v", err)
	}
	if captured.path != "/jobs/12345" {
		t.Fatalf("path = %q, want /jobs/12345", captured.path)
	}
	if !job.Auth.Enable || job.Auth.User != "u" {
		t.Fatalf("Auth = %+v", job.Auth)
	}
	if !job.Notification.OnFailure || job.Notification.OnFailureCount != 2 {
		t.Fatalf("Notification = %+v", job.Notification)
	}
	if job.ExtendedData.Headers["X-Foo"] != "Bar" || job.ExtendedData.Body != "Hello World!" {
		t.Fatalf("ExtendedData = %+v", job.ExtendedData)
	}
	if job.JobID != 12345 || job.RequestTimeout != 300 {
		t.Fatalf("embedded Job fields = %+v", job.Job)
	}
}

func TestCreateJobWrapsPayloadAndReturnsID(t *testing.T) {
	client, captured := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"jobId":12345}`)
	})

	title := "kiokku reminder sweep"
	url := "https://kiokku.fly.dev/api/v1/reminders/sweep"
	enabled := true
	jobID, err := client.CreateJob(context.Background(), JobInput{
		URL:      &url,
		Title:    &title,
		Enabled:  &enabled,
		Schedule: MustEveryNMinutes(15, "UTC"),
	})
	if err != nil {
		t.Fatalf("CreateJob: %v", err)
	}
	if jobID != 12345 {
		t.Fatalf("jobID = %d, want 12345", jobID)
	}
	if captured.method != http.MethodPut || captured.path != "/jobs" {
		t.Fatalf("request = %s %s, want PUT /jobs", captured.method, captured.path)
	}
	// The API ignores the payload unless this header is exactly this value.
	if captured.contentType != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", captured.contentType)
	}

	var sent struct {
		Job map[string]any `json:"job"`
	}
	if err := json.Unmarshal([]byte(captured.body), &sent); err != nil {
		t.Fatalf("decode sent body: %v", err)
	}
	if sent.Job["url"] != url || sent.Job["title"] != title {
		t.Fatalf("sent job = %v", sent.Job)
	}
	// Fields the caller never set must be absent, not zero values.
	for _, key := range []string{"saveResponses", "folderId", "requestMethod", "auth", "notification"} {
		if _, present := sent.Job[key]; present {
			t.Fatalf("unset field %q was sent: %v", key, sent.Job)
		}
	}
}

func TestCreateJobRejectsBadURLBeforeSpendingQuota(t *testing.T) {
	called := false
	client, _ := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		called = true
		_, _ = io.WriteString(w, `{"jobId":1}`)
	})

	missingURL := JobInput{Title: pointerTo("no url")}
	if _, err := client.CreateJob(context.Background(), missingURL); err == nil {
		t.Fatal("expected an error when URL is missing")
	}

	badScheme := JobInput{URL: pointerTo("ftp://example.com/")}
	if _, err := client.CreateJob(context.Background(), badScheme); err == nil {
		t.Fatal("expected an error for a non-http scheme")
	}
	if called {
		t.Fatal("the client called the API despite invalid input")
	}
}

// UpdateJob must send exactly the fields that were set: a PATCH carrying zero
// values for untouched fields would clear settings the caller never named.
func TestUpdateJobSendsOnlyTheDelta(t *testing.T) {
	client, captured := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{}`)
	})

	if err := client.SetJobEnabled(context.Background(), 12345, false); err != nil {
		t.Fatalf("SetJobEnabled: %v", err)
	}
	if captured.method != http.MethodPatch || captured.path != "/jobs/12345" {
		t.Fatalf("request = %s %s, want PATCH /jobs/12345", captured.method, captured.path)
	}
	if captured.body != `{"job":{"enabled":false}}` {
		t.Fatalf("body = %s, want only the enabled delta", captured.body)
	}
}

func TestUpdateJobRejectsEmptyDelta(t *testing.T) {
	called := false
	client, _ := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		called = true
	})

	if err := client.UpdateJob(context.Background(), 12345, JobInput{}); err == nil {
		t.Fatal("expected an error for an empty delta")
	}
	if called {
		t.Fatal("the client spent a request on an empty delta")
	}
}

func TestDeleteJobSendsNoBody(t *testing.T) {
	client, captured := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{}`)
	})

	if err := client.DeleteJob(context.Background(), 12345); err != nil {
		t.Fatalf("DeleteJob: %v", err)
	}
	if captured.method != http.MethodDelete || captured.path != "/jobs/12345" {
		t.Fatalf("request = %s %s, want DELETE /jobs/12345", captured.method, captured.path)
	}
	if captured.body != "" {
		t.Fatalf("body = %q, want empty", captured.body)
	}
}

func TestHistoryDecodesItemsAndPredictions(t *testing.T) {
	client, captured := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"history":[{"jobLogId":4946,"jobId":12345,"identifier":"12345-22-11-4946","date":1640189711,"duration":239,"status":1,"statusText":"OK","httpStatus":200,"headers":null,"body":null,"stats":{"nameLookup":1003,"total":85548}}],"predictions":[1640189760,1640189820]}`)
	})

	history, err := client.GetHistory(context.Background(), 12345)
	if err != nil {
		t.Fatalf("GetHistory: %v", err)
	}
	if captured.path != "/jobs/12345/history" {
		t.Fatalf("path = %q", captured.path)
	}
	if len(history.Items) != 1 || len(history.Predictions) != 2 {
		t.Fatalf("history = %+v", history)
	}
	item := history.Items[0]
	if item.Identifier != "12345-22-11-4946" || item.HTTPStatus != 200 {
		t.Fatalf("item = %+v", item)
	}
	// The list endpoint never populates these, and nil says "not fetched"
	// while "" would claim the host answered with an empty body.
	if item.Headers != nil || item.Body != nil {
		t.Fatalf("Headers/Body = %v/%v, want nil", item.Headers, item.Body)
	}
	if item.Stats.NameLookup != 1003 || item.Stats.Total != 85548 {
		t.Fatalf("stats = %+v", item.Stats)
	}
}

func TestGetHistoryItemEscapesIdentifier(t *testing.T) {
	client, captured := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"jobHistoryDetails":{"jobId":12345,"identifier":"12345-22-11-4946","body":"<!doctype html>"}}`)
	})

	item, err := client.GetHistoryItem(context.Background(), 12345, "12345-22-11-4946")
	if err != nil {
		t.Fatalf("GetHistoryItem: %v", err)
	}
	if captured.path != "/jobs/12345/history/12345-22-11-4946" {
		t.Fatalf("path = %q", captured.path)
	}
	if item.Body == nil || *item.Body != "<!doctype html>" {
		t.Fatalf("Body = %v", item.Body)
	}
}

func TestFolderCallsUseTheDocumentedShapes(t *testing.T) {
	client, captured := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/folders":
			_, _ = io.WriteString(w, `{"folders":[{"folderId":9876,"title":"Backups"}]}`)
		case r.Method == http.MethodPut:
			_, _ = io.WriteString(w, `{"folderId":9876}`)
		default:
			_, _ = io.WriteString(w, `{}`)
		}
	})

	folders, err := client.ListFolders(context.Background())
	if err != nil {
		t.Fatalf("ListFolders: %v", err)
	}
	if len(folders) != 1 || folders[0].FolderID != 9876 {
		t.Fatalf("folders = %+v", folders)
	}

	folderID, err := client.CreateFolder(context.Background(), "Backups")
	if err != nil {
		t.Fatalf("CreateFolder: %v", err)
	}
	if folderID != 9876 {
		t.Fatalf("folderID = %d", folderID)
	}
	if captured.body != `{"folder":{"title":"Backups"}}` {
		t.Fatalf("body = %s", captured.body)
	}

	if err := client.DeleteFolder(context.Background(), 9876); err != nil {
		t.Fatalf("DeleteFolder: %v", err)
	}
	if captured.path != "/folders/9876" {
		t.Fatalf("path = %q", captured.path)
	}
}

func TestErrorStatusesMapToSentinels(t *testing.T) {
	cases := []struct {
		status int
		want   error
	}{
		{400, ErrBadRequest},
		{401, ErrUnauthorized},
		{403, ErrForbidden},
		{404, ErrNotFound},
		{409, ErrConflict},
		{429, ErrTooManyRequests},
		{500, ErrServer},
		{503, ErrServer},
	}

	for _, testCase := range cases {
		client, _ := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(testCase.status)
			_, _ = io.WriteString(w, `{"error":"nope"}`)
		})

		_, _, err := client.ListJobs(context.Background())
		if !errors.Is(err, testCase.want) {
			t.Fatalf("status %d: err = %v, want %v", testCase.status, err, testCase.want)
		}

		var apiErr *APIError
		if !errors.As(err, &apiErr) {
			t.Fatalf("status %d: err is not an *APIError", testCase.status)
		}
		if apiErr.StatusCode != testCase.status {
			t.Fatalf("StatusCode = %d, want %d", apiErr.StatusCode, testCase.status)
		}
	}
}

// 429 means quota OR rate limit, so the body and the Retry-After hint must
// survive: a caller that sees only the sentinel cannot tell a wait-a-second
// from a wait-until-tomorrow.
func TestTooManyRequestsKeepsBodyAndRetryAfter(t *testing.T) {
	client, _ := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "60")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = io.WriteString(w, `{"error":"daily quota exceeded"}`)
	})

	_, _, err := client.ListJobs(context.Background())

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("err is not an *APIError: %v", err)
	}
	if !strings.Contains(apiErr.Message, "daily quota exceeded") {
		t.Fatalf("Message = %q, want the body", apiErr.Message)
	}
	if apiErr.RetryAfter != "60" {
		t.Fatalf("RetryAfter = %q, want 60", apiErr.RetryAfter)
	}
}

func TestAPIErrorNeverLeaksTheKeyAndTruncatesTheBody(t *testing.T) {
	client, _ := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = io.WriteString(w, strings.Repeat("x", maxErrorBodyBytes*3))
	})

	_, _, err := client.ListJobs(context.Background())
	message := err.Error()
	if strings.Contains(message, testAPIKey) {
		t.Fatal("the API key appeared in an error message")
	}
	if len(message) > maxErrorBodyBytes*2 {
		t.Fatalf("error message is %d bytes, want it truncated", len(message))
	}
	if !strings.Contains(message, "truncated") {
		t.Fatal("a cut body must say so")
	}
}

func TestScheduleHelpers(t *testing.T) {
	quarterly := MustEveryNMinutes(15, "Asia/Ho_Chi_Minh")
	if got := quarterly.Minutes; len(got) != 4 || got[0] != 0 || got[3] != 45 {
		t.Fatalf("Minutes = %v, want [0 15 30 45]", got)
	}
	// Every other unit must be the wildcard, or the job runs only in the hour
	// or month whose value happens to be set.
	for name, values := range map[string][]int{
		"hours": quarterly.Hours, "mdays": quarterly.MDays,
		"months": quarterly.Months, "wdays": quarterly.WDays,
	} {
		if len(values) != 1 || values[0] != everyValue {
			t.Fatalf("%s = %v, want [-1]", name, values)
		}
	}
	if err := quarterly.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}

	if _, err := EveryNMinutes(7, "UTC"); err == nil {
		t.Fatal("expected an error for a non-divisor of 60")
	}

	daily, err := Daily(6, 30, "UTC")
	if err != nil {
		t.Fatalf("Daily: %v", err)
	}
	if daily.Hours[0] != 6 || daily.Minutes[0] != 30 {
		t.Fatalf("Daily = %+v", daily)
	}
	if _, err := Daily(24, 0, "UTC"); err == nil {
		t.Fatal("expected an error for hour 24")
	}

	weekly, err := Weekly(time.Monday, 8, 0, "UTC")
	if err != nil {
		t.Fatalf("Weekly: %v", err)
	}
	if len(weekly.WDays) != 1 || weekly.WDays[0] != int(time.Monday) {
		t.Fatalf("WDays = %v", weekly.WDays)
	}

	if minute := EveryMinute("UTC").Minutes; len(minute) != 1 || minute[0] != everyValue {
		t.Fatalf("EveryMinute Minutes = %v", minute)
	}
}

// Two schedules from the helpers must not share a backing array, or editing one
// job's schedule would silently edit another's.
func TestScheduleHelpersDoNotShareSlices(t *testing.T) {
	first := EveryMinute("UTC")
	second := EveryMinute("UTC")
	first.Hours[0] = 5
	if second.Hours[0] != everyValue {
		t.Fatalf("second.Hours = %v, want [-1] — the helpers alias a shared slice", second.Hours)
	}
}

func TestScheduleValidateRejectsUnrunnableShapes(t *testing.T) {
	empty := Schedule{Hours: []int{1}, MDays: []int{1}, Minutes: nil, Months: []int{1}, WDays: []int{1}}
	if err := empty.Validate(); err == nil {
		t.Fatal("expected an error for an empty unit")
	}

	mixed := *EveryMinute("UTC")
	mixed.Hours = []int{everyValue, 5}
	if err := mixed.Validate(); err == nil {
		t.Fatal("expected an error when -1 is mixed with explicit values")
	}

	outOfRange := *EveryMinute("UTC")
	outOfRange.Minutes = []int{60}
	if err := outOfRange.Validate(); err == nil {
		t.Fatal("expected an error for minute 60")
	}
}

func TestExpiresAtPacksTheDecimalFormat(t *testing.T) {
	if got := ExpiresAt(time.Time{}); got != 0 {
		t.Fatalf("zero time = %d, want 0", got)
	}
	moment := time.Date(2026, time.September, 18, 7, 5, 9, 0, time.UTC)
	if got := ExpiresAt(moment); got != 20260918070509 {
		t.Fatalf("ExpiresAt = %d, want 20260918070509", got)
	}
}

func TestContextCancellationPropagates(t *testing.T) {
	client, _ := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"jobs":[],"someFailed":false}`)
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, _, err := client.ListJobs(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}

// pointerTo is the local test spelling of kuery/conv.ToPointer, kept here so
// the package's own tests stay standard-library-only like the package.
func pointerTo[T any](value T) *T { return &value }
