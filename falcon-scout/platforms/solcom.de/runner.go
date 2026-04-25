package solcomde

import (
	"context"
	"fmt"
	"time"

	"hblabs.co/falcon/scout/platformkit"
)

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
func (r *Runner) SetSaveHandler(fn platformkit.SaveFn)     { r.save = fn }
func (r *Runner) SetFilterHandler(fn platformkit.FilterFn) { r.filter = fn }
func (r *Runner) SetWarnHandler(fn platformkit.WarnFn)     { r.warn = fn }
func (r *Runner) SetErrHandler(fn platformkit.ErrFn)       { r.err = fn }

func (r *Runner) SetBatchConfig(cfg platformkit.BatchConfig) {
	r.batchFn = platformkit.ThrottledBatch[*ProjectCandidate](cfg)
}

func (r *Runner) Init(ctx context.Context) error { return nil }
func (r *Runner) StartConsumers(ctx context.Context) error {
	consumer := platformkit.ResolveConsumerName(Source)
	r.logger.Warnf("subscription to → %s not completed", consumer)
	return nil
}
func (r *Runner) StartWorkers(ctx context.Context) {
	r.logger.Info("workers started")
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
	platformkit.Order(&toFetch, true)
	return toFetch, existing, nil
}

// collectCandidates fetches the RSS feed (single HTTP GET, no colly), parses
// all items, and filters out already-known ones. No pagination — the feed
// contains all active projects.
func (r *Runner) collectCandidates(ctx context.Context) ([]*ProjectCandidate, map[string]any, error) {
	existing := make(map[string]any)

	items, err := fetchFeed(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("fetch rss feed: %w", err)
	}

	// Convert RSS items to candidates.
	var candidates []*ProjectCandidate
	for _, item := range items {
		c := itemToCandidate(item)
		if c != nil {
			candidates = append(candidates, c)
		}
	}

	if len(candidates) == 0 {
		r.logger.Warnf("rss feed returned %d items but 0 parseable candidates", len(items))
		if r.err != nil {
			emptyErr := &platformkit.ErrEmptyListing{Page: 1, CardsSeen: len(items)}
			name, priority, opts := platformkit.ClassifyError(emptyErr)
			_ = r.err(ctx, name, emptyErr.Error(), priority, "", nil, opts...)
		}
		return nil, existing, nil
	}

	// Apply filter if available.
	if r.filter != nil {
		updatedAt := make(map[string]time.Time, len(candidates))
		for _, c := range candidates {
			updatedAt[c.PlatformID] = parseCandidateDate(c.PublishedAt)
		}

		skip, pageExisting, filterErr := r.filter(ctx, Source, updatedAt)
		if filterErr != nil {
			r.logger.Errorf("filter candidates: %v", filterErr)
			return nil, existing, nil
		}

		for id, ref := range pageExisting {
			existing[id] = ref
		}

		var filtered []*ProjectCandidate
		for _, c := range candidates {
			if !skip[c.PlatformID] {
				filtered = append(filtered, c)
			}
		}

		r.logger.Infof("%d new, %d skipped (of %d feed items)", len(filtered), len(candidates)-len(filtered), len(items))
		candidates = filtered
	}

	return candidates, existing, nil
}

// #############################################################################

// process for solcom is simpler than HTML scrapers — all data is already in
// the RSS feed. No detail page visit needed. We just build the Project from
// the candidate and save it.
func (r *Runner) process(ctx context.Context, c *ProjectCandidate, existing any) {
	r.logger.Infof("processing %s", c.PlatformID)

	project := candidateToProject(c)

	if r.save != nil {
		if err := r.save(ctx, project, existing); err != nil {
			log := r.logger.WithFields(map[string]any{
				"platform_id": c.PlatformID,
				"url":         c.URL,
			})
			log.Errorf("save failed: %v", err)
		}
	}
}

// candidateToProject builds a Project from the RSS-extracted candidate data.
// No inspect step needed — the feed has everything.
func candidateToProject(c *ProjectCandidate) *Project {
	return &Project{
		PlatformID:  c.PlatformID,
		URL:         c.URL,
		Title:       c.Title,
		Description: c.Description,
		Location:    c.Location,
		StartDate:   c.StartDate,
		Duration:    c.Duration,
		Remote:      c.Remote,
		PublishedAt: c.PublishedAt,
		Rate:        "Auf Anfrage",
		ScrapedAt:   time.Now(),
	}
}

// Retry re-processes a previously failed candidate from the errors collection.
func (r *Runner) Retry(ctx context.Context, rawCandidate any, existing any) error {
	c, err := decodeCandidate(rawCandidate)
	if err != nil {
		return fmt.Errorf("decode candidate: %w", err)
	}

	r.logger.Infof("[retry] re-processing %s", c.PlatformID)

	project := candidateToProject(c)
	if r.save != nil {
		if err := r.save(ctx, project, existing); err != nil {
			return fmt.Errorf("save after retry: %w", err)
		}
	}

	r.logger.Infof("[retry] successfully re-processed %s", c.PlatformID)
	return nil
}

// parseCandidateDate parses a canonical YYYY-MM-DD date into a UTC time.Time.
func parseCandidateDate(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t.UTC()
	}
	return time.Time{}
}
