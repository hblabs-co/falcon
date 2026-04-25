package contractorde

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

func (r *Runner) SetBatchConfig(cfg platformkit.BatchConfig) {
	r.batchFn = platformkit.ThrottledBatch[*ProjectCandidate](cfg)
}

func (r *Runner) Init(ctx context.Context) error {
	return nil
}

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
	// Process newest projects first: contractor.de PlatformIDs are
	// sequential integers, so highest ID = most recent posting.
	platformkit.OrderBy(&toFetch, func(c *ProjectCandidate) string { return c.PlatformID }, true)
	return toFetch, existing, nil
}

// collectCandidates scrapes the single listing page and filters out already
// known candidates. No pagination — contractor.de shows all projects at once.
func (r *Runner) collectCandidates(ctx context.Context) ([]*ProjectCandidate, map[string]any, error) {
	existing := make(map[string]any)

	candidates, html, cardsSeen, err := scrapeListing()
	if err != nil {
		return nil, nil, fmt.Errorf("scrape listing: %w", err)
	}

	// Same empty-listing detection as redglobal.
	if len(candidates) == 0 {
		emptyErr := &platformkit.ErrEmptyListing{Page: 1, HTML: html, CardsSeen: cardsSeen}
		r.logger.Errorf("listing returned no candidates — possible markup drift (cards seen: %d)", cardsSeen)
		if r.err != nil {
			name, priority, opts := platformkit.ClassifyError(emptyErr)
			_ = r.err(ctx, name, emptyErr.Error(), priority, html, nil, opts...)
		}
		return nil, existing, nil
	}

	// Apply filter if available.
	if r.filter != nil {
		// contractor.de has no publication date (StartDate is when the project
		// starts, not when it was posted), so we emit zero for every candidate.
		// The filter then matches it against the zero persisted as PlatformUpdatedAt
		// and skips known projects on subsequent polls.
		updatedAt := make(map[string]time.Time, len(candidates))
		for _, c := range candidates {
			updatedAt[c.PlatformID] = time.Time{}
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

		r.logger.Infof("%d new, %d skipped", len(filtered), len(candidates)-len(filtered))
		candidates = filtered
	}

	return candidates, existing, nil
}

// #############################################################################

func (r *Runner) process(ctx context.Context, c *ProjectCandidate, existing any) {
	r.logger.Infof("processing %s", c.PlatformID)

	log := r.logger.WithFields(map[string]any{
		"platform_id": c.PlatformID,
		"current":     c.Current,
		"total":       c.Total,
		"url":         c.URL,
	})

	result, err := Inspect(c)
	if err != nil {
		if platformkit.IsGone(err) {
			log.Infof("project gone (410) — skipping")
			return
		}
		name, priority, opts := platformkit.ClassifyError(err)
		log.Warnf("inspect failed (%s): %v — recording for retry", name, err)
		if r.err != nil {
			_ = r.err(ctx, name, err.Error(), priority, "", c, opts...)
		}
		return
	}

	if r.save != nil {
		if err := r.save(ctx, result.Project, existing); err != nil {
			log.Errorf("save failed: %v", err)
		}
	}
}

// Retry re-processes a previously failed candidate from the errors collection.
func (r *Runner) Retry(ctx context.Context, rawCandidate any, existing any) error {
	c, err := decodeCandidate(rawCandidate)
	if err != nil {
		return fmt.Errorf("decode candidate: %w", err)
	}

	r.logger.Infof("[retry] re-inspecting %s (%s)", c.PlatformID, c.URL)

	result, err := Inspect(c)
	if err != nil {
		return err
	}

	if r.save != nil {
		if err := r.save(ctx, result.Project, existing); err != nil {
			return fmt.Errorf("save after retry: %w", err)
		}
	}

	r.logger.Infof("[retry] successfully re-scraped %s", c.PlatformID)
	return nil
}
