package somide

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

func (r *Runner) SetLogger(logger any)                       { r.logger = platformkit.ResolveLogger(logger) }
func (r *Runner) SetSaveHandler(fn platformkit.SaveFn)       { r.save = fn }
func (r *Runner) SetFilterHandler(fn platformkit.FilterFn)   { r.filter = fn }
func (r *Runner) SetWarnHandler(fn platformkit.WarnFn)       { r.warn = fn }
func (r *Runner) SetErrHandler(fn platformkit.ErrFn)         { r.err = fn }
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

// collectCandidates paginates through the API and filters out already-known
// candidates. Stops when a page is entirely known or when we've reached total.
func (r *Runner) collectCandidates(ctx context.Context) ([]*ProjectCandidate, map[string]any, error) {
	existing := make(map[string]any)
	var allCandidates []*ProjectCandidate
	seen := 0

	for page := 0; ; page++ {
		jobs, total, err := fetchPage(ctx, page)
		if err != nil {
			return nil, nil, fmt.Errorf("fetch page %d: %w", page, err)
		}
		if len(jobs) == 0 {
			if page == 0 {
				r.logger.Warnf("api page 0 returned 0 jobs (total: %d)", total)
				if r.err != nil {
					emptyErr := &platformkit.ErrEmptyListing{Page: 0, CardsSeen: 0}
					name, priority, opts := platformkit.ClassifyError(emptyErr)
					_ = r.err(ctx, name, emptyErr.Error(), priority, "", nil, opts...)
				}
			}
			break
		}

		// Convert jobs to candidates (non-freelance skipped).
		var pageCandidates []*ProjectCandidate
		for _, j := range jobs {
			if c := jobToCandidate(j); c != nil {
				pageCandidates = append(pageCandidates, c)
			}
		}

		seen += len(jobs)

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

			r.logger.Infof("page %d: %d new, %d skipped (of %d jobs, %d total)",
				page, len(filtered), len(pageCandidates)-len(filtered), len(jobs), total)

			if len(filtered) == 0 {
				break
			}

			allCandidates = append(allCandidates, filtered...)
		} else {
			allCandidates = append(allCandidates, pageCandidates...)
		}

		if seen >= total {
			break
		}
	}

	return allCandidates, existing, nil
}

// #############################################################################

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
