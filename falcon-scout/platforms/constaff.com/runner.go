package constaffcom

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/gocolly/colly/v2"
	"hblabs.co/falcon/scout/platformkit"
)

type Runner struct {
	logger  platformkit.Logger
	save    platformkit.SaveFn
	filter  platformkit.FilterFn
	warn    platformkit.WarnFn
	err     platformkit.ErrFn
	batchFn platformkit.BatchFn[*ProjectCandidate]

	// contacts is the cached Ansprechpartner directory, keyed by
	// uppercase initials (e.g. "JBA"). Populated on Init(), refreshed
	// daily by the worker goroutine.
	contacts   map[string]contactEntry
	contactsMu sync.RWMutex

	// ContactMode controls how contact info is resolved per project:
	//   "api"  (default) — Ansprechpartner directory, fallback to HTML scraping
	//   "html" — always scrape the detail page (for testing the HTML parser)
	ContactMode string

	// detailCollector is the shared colly collector for detail page
	// scraping. Created once in Init(), reused across all scrapes so
	// the rate limiter (Delay between requests) is respected globally.
	detailCollector *colly.Collector
}

func New() *Runner {
	return &Runner{
		logger:  platformkit.NoopLogger{},
		batchFn: platformkit.SequentialBatch[*ProjectCandidate],
	}
}

func (r *Runner) Name() string      { return Source }
func (r *Runner) BaseURL() string   { return baseURL }
func (r *Runner) CompanyID() string { return CompanyID }

func (r *Runner) SetLogger(logger any)                     { r.logger = platformkit.ResolveLogger(logger) }
func (r *Runner) SetSaveHandler(fn platformkit.SaveFn)     { r.save = fn }
func (r *Runner) SetFilterHandler(fn platformkit.FilterFn) { r.filter = fn }
func (r *Runner) SetWarnHandler(fn platformkit.WarnFn)     { r.warn = fn }
func (r *Runner) SetErrHandler(fn platformkit.ErrFn)       { r.err = fn }
func (r *Runner) SetBatchConfig(cfg platformkit.BatchConfig) {
	r.batchFn = platformkit.ThrottledBatch[*ProjectCandidate](cfg)
}
func (r *Runner) Init(ctx context.Context) error {
	// Shared collector with rate limiter — one request at a time, 1s+
	// random delay between requests. Prevents 503 on the slow server.
	r.detailCollector = newDetailCollector()

	// Synchronous load — guarantees contacts are ready before Poll starts.
	// StartWorker will re-fetch on its first tick (idempotent, no race).
	// r.refreshContacts(ctx)
	r.ContactMode = "html"
	return nil
}

func (r *Runner) StartConsumers(ctx context.Context) error {
	consumer := platformkit.ResolveConsumerName(Source)
	r.logger.Warnf("subscription to → %s not completed", consumer)
	return nil
}

func (r *Runner) StartWorkers(ctx context.Context) {
	// platformkit.StartWorker(ctx, 24*time.Hour, r.refreshContacts)
	r.logger.Info("workers started")
}

// refreshContacts re-fetches the full Ansprechpartner list so newly
// added team members are picked up without a service restart.
func (r *Runner) refreshContacts(ctx context.Context) {
	dir, err := fetchContactDirectory(ctx)
	if err != nil {
		r.logger.Warnf("contact directory refresh failed: %v", err)
		return
	}
	r.contactsMu.Lock()
	r.contacts = dir
	r.contactsMu.Unlock()
	r.logger.Infof("refreshed contact directory: %d entries", len(dir))
}

// resolveContact looks up a team member by their 3-letter initials
// (e.g. "JBA") and returns the full contact entry. Returns zero value
// when the initials aren't in the directory.
func (r *Runner) resolveContact(initials string) contactEntry {
	r.contactsMu.RLock()
	defer r.contactsMu.RUnlock()
	return r.contacts[strings.ToUpper(strings.TrimSpace(initials))]
}

// enrichContact populates the contact fields on the candidate. Strategy
// depends on ContactMode:
//
//   - "html"    → always scrape the detail page (for testing)
//   - otherwise → try directory cache first, scrape detail page as fallback
func (r *Runner) enrichContact(c *ProjectCandidate) {
	if r.ContactMode == "html" {
		r.enrichFromHTML(c)
		return
	}
	// Default: directory lookup, HTML fallback.
	if contact := r.resolveContact(c.ContactInitials); contact.Name != "" {
		applyContact(c, contact)
		return
	}
	r.enrichFromHTML(c)
}

func (r *Runner) enrichFromHTML(c *ProjectCandidate) {
	contact, err := scrapeContactFromDetailPage(r.detailCollector, c.PlatformID)
	if err != nil {
		r.logger.Warnf("contact scrape for %s: %v", c.PlatformID, err)
		return
	}
	applyContact(c, contact)
}

func applyContact(c *ProjectCandidate, contact contactEntry) {
	c.ContactName = contact.Name
	c.ContactPosition = contact.Position
	c.ContactPhone = contact.Phone
	c.ContactEmail = contact.Email
	c.ContactImage = contact.Image
}

func (r *Runner) Poll(ctx context.Context) func() {
	return func() {
		items, existing, err := r.findCandidates(ctx)
		if err != nil || len(items) == 0 {
			return
		}
		r.batchFn(ctx, items, func(ctx context.Context, item *ProjectCandidate) {
			r.process(ctx, item, existing[item.PlatformID])
		})
	}
}

func (r *Runner) findCandidates(ctx context.Context) ([]*ProjectCandidate, map[string]any, error) {
	toFetch, existing, err := r.collectCandidates(ctx)
	if err != nil {
		r.logger.Errorf("collect candidates: %v", err)
		return nil, nil, err
	}
	if len(toFetch) == 0 {
		r.logger.Info("no new or updated projects")
		return nil, nil, nil
	}
	// Process newest (highest Id) first — PlatformIDs are sequential integers.
	platformkit.OrderBy(&toFetch, func(c *ProjectCandidate) string { return c.PlatformID }, true)

	ids := make([]string, len(toFetch))
	for i, c := range toFetch {
		ids[i] = c.PlatformID
	}
	r.logger.Infof("sorted order: %v", ids)

	return toFetch, existing, nil
}

// collectCandidates paginates the admin-ajax endpoint (1-indexed) until
// we've seen `count` total items or the API returns an empty page. The
// per-page size isn't in the response body, so we rely on the total
// count and keep fetching until accumulated length reaches it.
func (r *Runner) collectCandidates(ctx context.Context) ([]*ProjectCandidate, map[string]any, error) {
	existing := make(map[string]any)
	var allCandidates []*ProjectCandidate
	seen := 0

	for page := 1; ; page++ {
		results, totalCount, err := fetchPage(ctx, page)
		if err != nil {
			return nil, nil, fmt.Errorf("fetch page %d: %w", page, err)
		}

		if len(results) == 0 {
			if page == 1 {
				r.logger.Warnf("api returned 0 results (total count: %d)", totalCount)
				if r.err != nil {
					emptyErr := &platformkit.ErrEmptyListing{Page: 1, CardsSeen: 0}
					name, priority, opts := platformkit.ClassifyError(emptyErr)
					_ = r.err(ctx, name, emptyErr.Error(), priority, "", nil, opts...)
				}
			}
			break
		}
		seen += len(results)

		// Convert API results to candidates (drops non-freelance types defensively).
		var pageCandidates []*ProjectCandidate
		for _, result := range results {
			if c := projectToCandidate(result); c != nil {
				pageCandidates = append(pageCandidates, c)
			}
		}

		// Apply filter.
		if r.filter != nil && len(pageCandidates) > 0 {
			updatedAt := make(map[string]time.Time, len(pageCandidates))
			for _, c := range pageCandidates {
				updatedAt[c.PlatformID] = parseCandidateDate(c.PostedAt)
			}

			skip, pageExisting, filterErr := r.filter(ctx, Source, updatedAt)
			if filterErr != nil {
				r.logger.Errorf("filter candidates: %v", filterErr)
				break
			}

			for id, ref := range pageExisting {
				existing[id] = ref
			}

			var filtered []*ProjectCandidate
			for _, c := range pageCandidates {
				if !skip[c.PlatformID] {
					filtered = append(filtered, c)
				}
			}

			r.logger.Infof("page %d: %d new, %d skipped (of %d results, %d total count)",
				page, len(filtered), len(pageCandidates)-len(filtered), len(results), totalCount)

			// Stop pagination when a full page is entirely known.
			if len(filtered) == 0 {
				break
			}
			allCandidates = append(allCandidates, filtered...)
		} else {
			allCandidates = append(allCandidates, pageCandidates...)
		}

		// Stop when we've seen every item the API reports.
		if seen >= totalCount {
			break
		}
	}

	return allCandidates, existing, nil
}

// #############################################################################

// process — all data comes from the listing API; no detail page fetch.
func (r *Runner) process(ctx context.Context, c *ProjectCandidate, existing any) {
	r.logger.Infof("processing %s", c.PlatformID)
	r.enrichContact(c)
	project := candidateToProject(c)
	if r.save != nil {
		if err := r.save(ctx, project, existing); err != nil {
			r.logger.WithFields(map[string]any{
				"platform_id": c.PlatformID,
				"url":         c.URL,
			}).Errorf("save failed: %v", err)
		}
	}
}

// Retry re-processes a previously failed candidate.
func (r *Runner) Retry(ctx context.Context, rawCandidate any, existing any) error {
	c, err := decodeCandidate(rawCandidate)
	if err != nil {
		return fmt.Errorf("decode candidate: %w", err)
	}
	r.logger.Infof("[retry] re-processing %s", c.PlatformID)
	r.enrichContact(c)
	project := candidateToProject(c)
	if r.save != nil {
		if err := r.save(ctx, project, existing); err != nil {
			return fmt.Errorf("save after retry: %w", err)
		}
	}
	r.logger.Infof("[retry] successfully re-processed %s", c.PlatformID)
	return nil
}

func parseCandidateDate(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t.UTC()
	}
	return time.Time{}
}
