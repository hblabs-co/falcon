package computerfuturescom

import (
	"context"
	"fmt"
	"time"

	"hblabs.co/falcon/modules/platformkit"
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

func (r *Runner) SetLogger(logger any)                     { r.logger = platformkit.ResolveLogger(logger) }
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
func (r *Runner) StartWorkers(ctx context.Context) { r.logger.Info("workers started") }

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

// collectCandidates paginates through the sthree API, collecting all freelance
// projects and filtering out already-known ones. The filter stops pagination
// when a full page is entirely known (same as redglobal's scrapeLoop).
func (r *Runner) collectCandidates(ctx context.Context) ([]*ProjectCandidate, map[string]any, error) {
	existing := make(map[string]any)
	var allCandidates []*ProjectCandidate

	for from := 0; ; from += pageSize {
		results, totalHits, err := fetchPage(ctx, from)
		if err != nil {
			return nil, nil, fmt.Errorf("fetch api page from=%d: %w", from, err)
		}

		if len(results) == 0 {
			if from == 0 {
				r.logger.Warnf("api returned 0 results (total hits: %d)", totalHits)
				if r.err != nil {
					emptyErr := &platformkit.ErrEmptyListing{Page: 1, CardsSeen: 0}
					name, priority, opts := platformkit.ClassifyError(emptyErr)
					_ = r.err(ctx, name, emptyErr.Error(), priority, "", nil, opts...)
				}
			}
			break
		}

		// Convert API results to candidates.
		var pageCandidates []*ProjectCandidate
		for _, result := range results {
			if c := resultToCandidate(result); c != nil {
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

			r.logger.Infof("page from=%d: %d new, %d skipped (of %d results, %d total hits)",
				from, len(filtered), len(pageCandidates)-len(filtered), len(results), totalHits)

			// Stop pagination when all items on a page are already known.
			if len(filtered) == 0 {
				break
			}

			allCandidates = append(allCandidates, filtered...)
		} else {
			allCandidates = append(allCandidates, pageCandidates...)
		}

		// Stop if we've reached the end.
		if from+len(results) >= totalHits {
			break
		}
	}

	return allCandidates, existing, nil
}

// #############################################################################

// process for computerfutures is simple — all data is already in the API
// response. No detail page visit needed.
func (r *Runner) process(ctx context.Context, c *ProjectCandidate, existing any) {
	r.logger.Infof("processing %s", c.PlatformID)
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
