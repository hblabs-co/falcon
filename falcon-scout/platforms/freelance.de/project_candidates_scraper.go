package freelancede

import (
	"context"
	"fmt"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

func ScrapeProjectCandidates(ctx context.Context) ([]*ProjectCandidate, error) {
	tctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	allocCtx, allocCancel := chromedp.NewExecAllocator(tctx,
		append(chromedp.DefaultExecAllocatorOptions[:],
			chromedp.Flag("headless", true), // non-headless for inspection
			chromedp.NoSandbox,
			chromedp.DisableGPU,
			// Skip images to speed up page load.
			chromedp.Flag("blink-settings", "imagesEnabled=false"),
		)...,
	)
	defer allocCancel()

	bCtx, bCancel := chromedp.NewContext(allocCtx)
	defer bCancel()

	// Inject session cookies before first navigation so the browser is already
	// authenticated when the listing page loads.
	var cookieParams []*network.CookieParam
	for _, c := range getSession().Cookies() {
		cookieParams = append(cookieParams, &network.CookieParam{
			Name:     c.Name,
			Value:    c.Value,
			Domain:   ".freelance.de",
			Path:     "/",
			Secure:   true,
			HTTPOnly: true,
		})
	}

	var rawCards []map[string]any

	err := chromedp.Run(bCtx,
		network.SetCookies(cookieParams),
		network.SetExtraHTTPHeaders(network.Headers{
			"Accept":                    "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7",
			"Accept-Encoding":           "gzip, deflate, br, zstd",
			"Accept-Language":           "de-DE,de;q=0.9",
			"Cache-Control":             "max-age=0",
			"User-Agent":                "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/146.0.0.0 Safari/537.36",
			"Sec-Ch-Ua":                 `"Chromium";v="146", "Not-A.Brand";v="24", "Google Chrome";v="146"`,
			"Sec-Ch-Ua-Mobile":          "?0",
			"Sec-Ch-Ua-Platform":        `"macOS"`,
			"Sec-Fetch-Dest":            "document",
			"Sec-Fetch-Mode":            "navigate",
			"Sec-Fetch-Site":            "same-origin",
			"Sec-Fetch-User":            "?1",
			"Priority":                  "u=0, i",
			"Upgrade-Insecure-Requests": "1",
		}),
		chromedp.Navigate(projectCandidatesURL),

		// Wait until at least one card is rendered.
		chromedp.Poll(`document.querySelectorAll('search-project-card > a[href*="projekt-"]').length > 0`, nil),

		// Extract all metadata from each search-project-card.
		chromedp.Evaluate(`
			Array.from(document.querySelectorAll('search-project-card')).map(card => {
				const getMetaSpan = cls =>
					card.querySelector('.' + cls)?.closest('li')?.querySelector('span')?.innerText?.trim() ?? '';

				const locDiv = card.querySelector('.fa-map-marker-alt')
					?.closest('li')?.querySelector('.d-flex.flex-column');
				const location = locDiv
					? Array.from(locDiv.querySelectorAll(':scope > span'))
						.map(s => s.innerText.trim()).filter(Boolean)
					: [];

				const skillsUl = card.querySelector('ul.list-inline-block');
				const skills = skillsUl
					? Array.from(skillsUl.querySelectorAll('a.badge'))
						.map(a => a.innerText.trim()).filter(Boolean)
					: [];

				return {
					url:               card.querySelector(':scope > a[href*="projekt-"]')?.href ?? '',
					title:             card.querySelector('h3')?.innerText?.trim() ?? '',
					company:           card.querySelector('small.text-secondary span')?.innerText?.trim() ?? '',
					companyLogo:       card.querySelector('img')?.src ?? '',
					skills,
					startDate:         getMetaSpan('fa-calendar-star'),
					location,
					remote:            !!card.querySelector('.fa-laptop-house'),
					platformUpdatedAt: getMetaSpan('fa-history'),
				};
			})
		`, &rawCards),
	)
	if err != nil {
		return nil, fmt.Errorf("project candidates scrape: %w", err)
	}

	now := time.Now()
	seen := make(map[string]bool)
	candidates := make([]*ProjectCandidate, 0, len(rawCards))
	for _, c := range rawCards {
		url, _ := c["url"].(string)
		id := platformIDRe.FindString(url)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true

		strSlice := func(key string) []string {
			raw, _ := c[key].([]any)
			out := make([]string, 0, len(raw))
			for _, v := range raw {
				if s, ok := v.(string); ok && s != "" {
					out = append(out, s)
				}
			}
			return out
		}
		str := func(key string) string { s, _ := c[key].(string); return s }

		target := ProjectCandidate{
			PlatformID:        id,
			URL:               url,
			Source:            Source,
			Title:             str("title"),
			Company:           str("company"),
			CompanyLogo:       str("companyLogo"),
			Skills:            strSlice("skills"),
			StartDate:         str("startDate"),
			Location:          strSlice("location"),
			Remote:            c["remote"] == true,
			PlatformUpdatedAt: str("platformUpdatedAt"),
			ScrapedAt:         now,
		}
		candidates = append(candidates, &target)
	}

	return candidates, nil
}
