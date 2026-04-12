package platformkit

// CallOptions holds the resolved flags after applying a list of CallOption
// functions. Service-side code reads this to decide how to route a recording
// (per-item insert vs categorical upsert, etc.).
type CallOptions struct {
	// Categorical marks the recording as a system-wide incident. The system
	// layer dedupes it via a deterministic ID (service:platform:name) and
	// upserts instead of inserting a fresh document. The candidate, if
	// provided, is preserved on the FIRST occurrence ($setOnInsert).
	Categorical bool
}

// CallOption is a functional option applied to ErrFn / WarnFn calls. The
// pattern lets the runner opt into behaviors like categorical aggregation
// without breaking existing call sites that pass none.
//
// A single Categorical() helper covers both errors and warnings because the
// flag has the same meaning in either context — the option type is shared.
type CallOption func(*CallOptions)

// Categorical marks the recording as a system-wide incident.
//
// Use it for problems where:
//   - The cause is a shared resource (auth, DNS, host, layout of common pages)
//   - The fix is applied once for the whole platform (re-deploy, re-auth)
//   - A second identical call would fail in the same way
//
// The opt is independent of whether you pass a candidate. A categorical
// recording with a candidate keeps the candidate as a "first observed example"
// — useful for forensics — without losing the dedup behavior.
func Categorical() CallOption {
	return func(o *CallOptions) { o.Categorical = true }
}

// ResolveOptions applies a list of CallOption to a fresh CallOptions struct
// and returns the resulting flags. Service-side code uses this to read the
// caller's intent before dispatching to the appropriate persistence path.
func ResolveOptions(opts []CallOption) CallOptions {
	var o CallOptions
	for _, fn := range opts {
		fn(&o)
	}
	return o
}
