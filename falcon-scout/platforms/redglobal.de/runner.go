package redglobalde

import (
	"context"
	"fmt"
	"maps"
	"time"

	"hblabs.co/falcon/modules/platformkit"
)

// implemnt retry
// capsulate more the logic so it gets more reusable

type Runner struct {
	logger  platformkit.Logger
	save    platformkit.SaveFn
	filter  platformkit.FilterFn
	warn    platformkit.WarnFn
	err     platformkit.ErrFn
	batchFn platformkit.BatchFn[*ProjectCandidate]
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

func (r *Runner) SetLogger(logger any) {
	r.logger = platformkit.ResolveLogger(logger)
}

func (r *Runner) SetSaveHandler(fn platformkit.SaveFn) {
	r.save = fn
}

func (r *Runner) SetFilterHandler(fn platformkit.FilterFn) {
	r.filter = fn
}

func (r *Runner) SetWarnHandler(fn platformkit.WarnFn) {
	r.warn = fn
}

func (r *Runner) SetErrHandler(fn platformkit.ErrFn) {
	r.err = fn
}

// SetBatchConfig swaps the default sequential batch processor for a throttled
// one driven by the values the service reads from the environment. This is what
// makes redglobal scrape at human pace instead of as fast as colly can return.
func (r *Runner) SetBatchConfig(cfg platformkit.BatchConfig) {
	r.batchFn = platformkit.ThrottledBatch[*ProjectCandidate](cfg)
}

func (r *Runner) Init(ctx context.Context) error {
	return nil
}

func (r *Runner) StartConsumers(ctx context.Context) error {

	// TODO: for api request to active scrape
	// subject := fmt.Sprintf("%s.%s", constants.SubjectScrapeRequested, Source)
	consumer := platformkit.ResolveConsumerName(Source)

	// TODO: can not call the system here
	// if err := system.Subscribe(ctx, constants.StreamScrape, consumer, subject, handleScrapeRequested); err != nil {
	// 	return fmt.Errorf("subscribe %s: %w", subject, err)
	// }
	// r.logger.Infof("subscribed → %s", subject)
	r.logger.Warnf("subscription to → %s not compled", consumer)

	return nil
}

func (r *Runner) StartWorkers(ctx context.Context) {
	r.logger.Info("workers started")
}

func (r *Runner) Poll(ctx context.Context) func() {
	return func() {
		items, existing, err := r.findCandiates(ctx)
		if err != nil || len(items) == 0 {
			return
		}
		// existing is a per-tick local map captured by the closure below.
		// No shared state on the Runner — concurrent ticks/workers are safe.
		r.batchFn(ctx, items, func(ctx context.Context, item *ProjectCandidate) {
			r.process(ctx, item, existing[item.PlatformID])
		})
	}
}

// ###############################################################################################
// ###############################################################################################
// ###############################################################################################

func (r *Runner) findCandiates(ctx context.Context) ([]*ProjectCandidate, map[string]any, error) {

	toFetch, existing, err := r.collectCandidates(ctx)
	if err != nil {
		r.logger.Errorf("collect candidates: %v", err)
		return nil, nil, err
	}

	total := len(toFetch)
	if total == 0 {
		r.logger.Info("no new or updated projects")
		return nil, nil, err
	}

	platformkit.Order(&toFetch, true)
	return toFetch, existing, nil
}

// collectCandidates scrapes listing pages starting from page 1.
// It filters out already-persisted or errored candidates and stops
// when all candidates on a page are already known. The existing map
// is per-tick local state — returned to the caller and propagated to
// process() so SaveFn can preserve identity without a second DB round-trip.
// No state is stored on the Runner, making concurrent ticks safe.
func (r *Runner) collectCandidates(ctx context.Context) ([]*ProjectCandidate, map[string]any, error) {
	existing := make(map[string]any)

	scraper := NewScraper()
	items, err := scraper.scrapeLoop(1, func(page int, found []*ProjectCandidate) ([]*ProjectCandidate, bool) {
		if r.filter == nil || len(found) == 0 {
			return found, false
		}

		// Build updatedAt map for the filter, parsing the listing date to time.Time.
		updatedAt := make(map[string]time.Time, len(found))
		for _, c := range found {
			updatedAt[c.PlatformID] = parseCandidateDate(c.PostedAt)
		}

		skip, pageExisting, err := r.filter(ctx, Source, updatedAt)
		if err != nil {
			r.logger.Errorf("filter candidates: %v", err)
			return nil, false
		}

		// Merge this page's existing records into the per-tick local map.
		maps.Copy(existing, pageExisting)

		var filtered []*ProjectCandidate
		for _, c := range found {
			if !skip[c.PlatformID] {
				filtered = append(filtered, c)
			}
		}

		r.logger.Infof("page %d: %d new, %d skipped", page, len(filtered), len(found)-len(filtered))

		// Continue if there are new candidates on this page.
		return filtered, len(filtered) > 0
	})

	// If page 1 returned zero candidates, the source platform almost certainly
	// changed its markup. Emit a categorical error so the dedup layer keeps a
	// single record (instead of one per poll) and the HTML snapshot is preserved
	// for diagnosis. The retry worker is not the right venue for this — there's
	// nothing to retry, only to investigate.
	if empty := platformkit.AsEmptyListing(err); empty != nil {
		r.logger.Errorf("listing page %d returned no candidates — possible markup drift", empty.Page)
		if r.err != nil {
			name, priority, opts := platformkit.ClassifyError(err)
			_ = r.err(ctx, name, err.Error(), priority, empty.HTML, nil, opts...)
		}
		// Treat the empty listing as "nothing to do this cycle" so the poll
		// loop continues. The categorical error stays in the DB until resolved.
		return nil, existing, nil
	}

	return items, existing, err
}

// ###############################################################################################
// ###############################################################################################

func (r *Runner) process(ctx context.Context, c *ProjectCandidate, existing any) {
	r.logger.Infof("processing %s", c.PlatformID)

	log := r.logger.WithFields(map[string]any{
		"platform_id": c.PlatformID,
		"current":     c.Current,
		"total":       c.Total,
		"url":         c.URL,
	})

	// // TEST_RETRY: simulate a 500 on the first item so the retry worker can
	// // pick it up. Fires once per process start — the env var is cleared after
	// // the first simulated failure. Remove the var to disable.
	// if os.Getenv("TEST_RETRY") != "" {
	// 	os.Unsetenv("TEST_RETRY")
	// 	log.Warnf("TEST_RETRY: simulating server error for %s", c.PlatformID)
	// 	if r.err != nil {
	// 		_ = r.err(ctx, platformkit.ErrNameScrapeServerError, "TEST_RETRY simulated 500", "medium", "", c)
	// 	}
	// 	return
	// }

	scraper := NewCandidateScraper(c)
	result, err := scraper.Inspect()
	if err != nil {
		// 410 Gone — the project was permanently removed. Skip silently, do not retry.
		if platformkit.IsGone(err) {
			log.Infof("project gone (410) — skipping")
			return
		}
		// Everything else: classify and record for the retry worker.
		name, priority, opts := platformkit.ClassifyError(err)
		log.Warnf("inspect failed (%s): %v — recording for retry", name, err)
		if r.err != nil {
			_ = r.err(ctx, name, err.Error(), priority, scraper.HTML, c, opts...)
		}
		return
	}

	// Reference ID is the human-facing identifier users quote when calling redglobal
	// about a specific job. If extraction failed, record a warning so we can spot
	// markup drift before it silently corrupts the dataset.
	if result.Project.ReferenceID == "" && r.warn != nil {
		message := "reference id not found on detail page — markup may have changed"
		_ = r.warn(ctx, platformkit.WarnReferenceIDNotFound, message, "high", result.HTML, c, platformkit.Categorical())
	}

	if r.save != nil {
		if err := r.save(ctx, result.Project, existing); err != nil {
			log.Errorf("save failed: %v", err)
		}
	}
}

// Retry re-processes a previously failed candidate from the errors collection.
// The retry worker in the service calls this after loading the error record and
// (optionally) the existing PersistedProject for the same platform_id.
func (r *Runner) Retry(ctx context.Context, rawCandidate any, existing any) error {
	c, err := decodeCandidate(rawCandidate)
	if err != nil {
		return fmt.Errorf("decode candidate: %w", err)
	}

	r.logger.Infof("[retry] re-inspecting %s (%s)", c.PlatformID, c.URL)

	scraper := NewCandidateScraper(c)
	result, err := scraper.Inspect()
	if err != nil {
		return err // retry worker handles retry_count / escalation
	}

	if r.save != nil {
		if err := r.save(ctx, result.Project, existing); err != nil {
			return fmt.Errorf("save after retry: %w", err)
		}
	}

	r.logger.Infof("[retry] successfully re-scraped %s", c.PlatformID)
	return nil
}
