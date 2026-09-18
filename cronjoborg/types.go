package cronjoborg

// JobStatus is the execution status of a job or of one history item.
type JobStatus int

// Job execution statuses as documented by the API.
const (
	StatusUnknown        JobStatus = 0 // not executed yet
	StatusOK             JobStatus = 1
	StatusFailedDNS      JobStatus = 2
	StatusFailedConnect  JobStatus = 3
	StatusFailedHTTP     JobStatus = 4
	StatusFailedTimeout  JobStatus = 5
	StatusFailedTooMuch  JobStatus = 6 // too much response data
	StatusFailedURL      JobStatus = 7 // invalid URL
	StatusFailedInternal JobStatus = 8
	StatusFailedUnknown  JobStatus = 9
)

// String returns the documented description of the status.
func (s JobStatus) String() string {
	switch s {
	case StatusUnknown:
		return "unknown"
	case StatusOK:
		return "ok"
	case StatusFailedDNS:
		return "failed (dns error)"
	case StatusFailedConnect:
		return "failed (could not connect to host)"
	case StatusFailedHTTP:
		return "failed (http error)"
	case StatusFailedTimeout:
		return "failed (timeout)"
	case StatusFailedTooMuch:
		return "failed (too much response data)"
	case StatusFailedURL:
		return "failed (invalid url)"
	case StatusFailedInternal:
		return "failed (internal errors)"
	case StatusFailedUnknown:
		return "failed (unknown reason)"
	default:
		return "unrecognized"
	}
}

// OK reports whether the status is a successful execution. Anything other than
// StatusOK is a failure, except StatusUnknown, which means the job has not run
// yet — test for that separately rather than treating it as a failure.
func (s JobStatus) OK() bool { return s == StatusOK }

// JobType is the kind of job. It is read-only on the API.
type JobType int

// Job types as documented by the API.
const (
	TypeDefault    JobType = 0
	TypeMonitoring JobType = 1 // used in a status monitor
)

// RequestMethod is the HTTP method the scheduler uses when it calls the job URL.
type RequestMethod int

// Request methods as documented by the API. The zero value is GET, which is
// also the API's default for an omitted field.
const (
	MethodGET     RequestMethod = 0
	MethodPOST    RequestMethod = 1
	MethodOPTIONS RequestMethod = 2
	MethodHEAD    RequestMethod = 3
	MethodPUT     RequestMethod = 4
	MethodDELETE  RequestMethod = 5
	MethodTRACE   RequestMethod = 6
	MethodCONNECT RequestMethod = 7
	MethodPATCH   RequestMethod = 8
)

// String returns the HTTP method name.
func (m RequestMethod) String() string {
	switch m {
	case MethodGET:
		return "GET"
	case MethodPOST:
		return "POST"
	case MethodOPTIONS:
		return "OPTIONS"
	case MethodHEAD:
		return "HEAD"
	case MethodPUT:
		return "PUT"
	case MethodDELETE:
		return "DELETE"
	case MethodTRACE:
		return "TRACE"
	case MethodCONNECT:
		return "CONNECT"
	case MethodPATCH:
		return "PATCH"
	default:
		return "UNKNOWN"
	}
}

// Schedule is the execution schedule of a job. Each array holds the values the
// job runs at; a single -1 means "every" (every hour, every minute, …), and an
// empty array means the job never runs on its own. The schedule helpers in
// schedule.go build the common shapes.
type Schedule struct {
	// Timezone is the schedule time zone as a PHP time-zone name (e.g. "UTC",
	// "Asia/Ho_Chi_Minh"). The API defaults to UTC when omitted.
	Timezone string `json:"timezone,omitempty"`
	// ExpiresAt is the date/time in the job's own time zone after which the job
	// is no longer scheduled, formatted YYYYMMDDhhmmss. 0 means it never
	// expires. It is a packed decimal, not a Unix timestamp.
	ExpiresAt int64 `json:"expiresAt"`
	// Hours to execute in (0-23; [-1] = every hour).
	Hours []int `json:"hours"`
	// MDays are days of month to execute on (1-31; [-1] = every day of month).
	MDays []int `json:"mdays"`
	// Minutes to execute at (0-59; [-1] = every minute).
	Minutes []int `json:"minutes"`
	// Months to execute in (1-12; [-1] = every month).
	Months []int `json:"months"`
	// WDays are days of week to execute on (0=Sunday … 6=Saturday;
	// [-1] = every day of week).
	WDays []int `json:"wdays"`
}

// Auth holds the HTTP basic authentication settings the scheduler uses when it
// calls the job URL.
type Auth struct {
	Enable   bool   `json:"enable"`
	User     string `json:"user,omitempty"`
	Password string `json:"password,omitempty"`
}

// Notification holds the per-job notification settings.
type Notification struct {
	// OnFailure sends a notification when the job fails.
	OnFailure bool `json:"onFailure"`
	// OnFailureCount is how many consecutive failures are required before a
	// notification is sent (min 1).
	OnFailureCount int `json:"onFailureCount,omitempty"`
	// OnSuccess sends a notification when the job succeeds after a failure.
	OnSuccess bool `json:"onSuccess"`
	// OnDisable sends a notification when the job is disabled automatically.
	OnDisable bool `json:"onDisable"`
	// OnSslCertExpiry sends a notification before the target's TLS certificate
	// expires.
	OnSslCertExpiry bool `json:"onSslCertExpiry"`
	// OnSslCertExpirySeconds is how long before expiry to notify (min 0; the
	// API defaults to 604800, i.e. 7 days).
	OnSslCertExpirySeconds int `json:"onSslCertExpirySeconds,omitempty"`
}

// ExtendedData holds the request headers and body the scheduler sends to the
// job URL.
type ExtendedData struct {
	Headers map[string]string `json:"headers,omitempty"`
	Body    string            `json:"body,omitempty"`
}

// Job is a cron job as returned by ListJobs. Read-only fields (everything from
// LastStatus through Type) are reported by the API and ignored on write.
type Job struct {
	JobID           int64         `json:"jobId"`
	Enabled         bool          `json:"enabled"`
	Title           string        `json:"title"`
	SaveResponses   bool          `json:"saveResponses"`
	URL             string        `json:"url"`
	LastStatus      JobStatus     `json:"lastStatus"`
	LastDuration    int           `json:"lastDuration"`  // milliseconds
	LastExecution   int64         `json:"lastExecution"` // unix seconds
	SslCertExpiry   int64         `json:"sslCertExpiry"` // unix seconds, HTTPS jobs only
	NextExecution   *int64        `json:"nextExecution"` // unix seconds; nil when the API has no prediction
	Type            JobType       `json:"type"`
	RequestTimeout  int           `json:"requestTimeout"` // seconds; -1 = API default
	RedirectSuccess bool          `json:"redirectSuccess"`
	FolderID        int64         `json:"folderId"`
	Schedule        Schedule      `json:"schedule"`
	RequestMethod   RequestMethod `json:"requestMethod"`
}

// DetailedJob is a Job plus the settings the API only returns for a single job
// (GetJob), not for the list.
type DetailedJob struct {
	Job
	Auth         Auth         `json:"auth"`
	Notification Notification `json:"notification"`
	ExtendedData ExtendedData `json:"extendedData"`
}

// JobInput is the write shape of a job, used by both CreateJob and UpdateJob.
//
// Every field is a pointer with omitempty because UpdateJob sends a DELTA: the
// API applies the fields that are present and leaves out the rest untouched.
// With value fields, a false or a 0 would be indistinguishable from "not set"
// and would silently clear settings the caller never meant to touch. Set only
// what should change; build the pointers with kuery/conv.ToPointer.
//
// URL is the only field the API requires on create.
type JobInput struct {
	Enabled         *bool          `json:"enabled,omitempty"`
	Title           *string        `json:"title,omitempty"`
	SaveResponses   *bool          `json:"saveResponses,omitempty"`
	URL             *string        `json:"url,omitempty"`
	RequestTimeout  *int           `json:"requestTimeout,omitempty"`
	RedirectSuccess *bool          `json:"redirectSuccess,omitempty"`
	FolderID        *int64         `json:"folderId,omitempty"`
	Schedule        *Schedule      `json:"schedule,omitempty"`
	RequestMethod   *RequestMethod `json:"requestMethod,omitempty"`
	Auth            *Auth          `json:"auth,omitempty"`
	Notification    *Notification  `json:"notification,omitempty"`
	ExtendedData    *ExtendedData  `json:"extendedData,omitempty"`
}

// Folder groups jobs in the account.
type Folder struct {
	FolderID int64  `json:"folderId"`
	Title    string `json:"title"`
}
