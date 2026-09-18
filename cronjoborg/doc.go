// Package cronjoborg is a typed, dependency-clean Go SDK for the cron-job.org
// REST API (https://docs.cron-job.org/rest-api.html).
//
// It exists because a service whose machine autostops cannot schedule its own
// recurring work: an in-process scheduler (see kuery/scheduler) stops when the
// machine stops, so an EXTERNAL scheduler is what both triggers the work and
// wakes the machine. This package lets such a service manage those external
// schedules itself — create, retarget, pause and inspect them — instead of
// depending on jobs a human clicked together in the console.
//
// Wire contract: base https://api.cron-job.org, JSON in and out,
// "Authorization: Bearer <api-key>" on every call, Content-Type
// application/json on every call that carries a body (the API IGNORES the
// payload when the header is missing or different). Every response is a plain
// object keyed by the resource — {"jobs": …}, {"jobDetails": …},
// {"jobId": …}, {"history": …, "predictions": …} — not an envelope; the SDK
// unwraps the key and returns the typed value.
//
// # Reading versus writing
//
// Reads return [Job] / [DetailedJob] with value fields. Writes take [JobInput],
// whose fields are all pointers with omitempty, because PATCH /jobs/{id} is a
// DELTA: a field that is present is applied, a field that is absent is left
// alone. Value fields cannot express that difference — a false or a 0 would be
// indistinguishable from "not set" and would silently clear settings. Build a
// JobInput with kuery/conv.ToPointer:
//
//	input := cronjoborg.JobInput{
//		URL:      conv.ToPointer("https://kiokku.fly.dev/api/v1/reminders/sweep"),
//		Title:    conv.ToPointer("kiokku reminder sweep"),
//		Enabled:  conv.ToPointer(true),
//		Schedule: cronjoborg.MustEveryNMinutes(15, "UTC"),
//	}
//
// # Rate limits and quotas
//
// A daily request quota applies to the whole key (100/day by default, 5,000/day
// for sustaining members), on top of per-method rate limits: CreateJob is the
// strict one at 1 request per second AND 5 per minute, folder writes are 1 per
// second, everything else 5 per second.
//
// Two status codes do not mean what they look like, and both have already cost
// a debugging session:
//
//   - 429 covers THREE different conditions — API-key quota, resource quota and
//     rate limit — with the same code. Retry logic that reads only the status
//     will back off forever against a daily quota it can never drain. Inspect
//     [APIError.Message] (the raw body) before deciding to retry. The SDK never
//     retries on its own, deliberately.
//   - 403 means "this API key cannot be used from this origin" (the key's IP
//     allowlist), NOT a bad key. Reacting to it by minting a new key chases the
//     wrong thing; 401 is the bad-key code.
//
// # Secrets
//
// The API key is a password: it grants full access to the cron-job.org account.
// Store it the way every other secret on this platform is stored (fly secrets /
// the service's env), never in a repository file, and prefer the console's IP
// restriction. The SDK sends the key on the Authorization header only and never
// puts it in an error message.
//
// The package imports only the standard library, so it stays the single source
// of truth for the contract and adds nothing to any consumer's dependency tree.
package cronjoborg
