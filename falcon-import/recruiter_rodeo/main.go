package main

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/v2/bson"
	"hblabs.co/falcon/packages/constants"
	"hblabs.co/falcon/packages/models"
	"hblabs.co/falcon/packages/system"
)

// entry holds the stats parsed from the HTML for one company.
type entry struct {
	companyName        string
	overallRating      float64
	recommendationRate string
	reviewCount        int
}

func main() {
	system.LoadEnvs()
	system.ConfigLogger()
	system.Init()

	ctx := context.Background()
	if err := system.InitStorage(ctx); err != nil {
		logrus.Fatalf("storage: %v", err)
	}

	htmlPath := resolveHTMLPath()
	logrus.Infof("reading HTML from %s", htmlPath)

	entries, err := parseHTML(htmlPath)
	if err != nil {
		logrus.Fatalf("parse html: %v", err)
	}
	logrus.Infof("parsed %d companies from HTML", len(entries))

	// Build lookup: normalized name → entry
	lookup := make(map[string]entry, len(entries))
	for _, e := range entries {
		lookup[normalizeName(e.companyName)] = e
	}

	// Paginate through all companies in MongoDB, collect matches, bulk write per page.
	const pageSize = 100
	page := 1
	updated, skipped := 0, 0

	for {
		var companies []models.Company
		total, err := system.GetStorage().FindPage(
			ctx, constants.MongoCompaniesCollection,
			bson.M{}, "created_at", false,
			page, pageSize, &companies,
		)
		if err != nil {
			logrus.Fatalf("find page %d: %v", page, err)
		}

		var bulk []system.UpsertDoc
		for i := range companies {
			c := &companies[i]
			matched, ok := findMatch(normalizeName(c.CompanyName), lookup)
			if !ok {
				skipped++
				continue
			}

			c.RecruiterRodeoStats = &models.RecruiterRodeoStats{
				OverallRating:      matched.overallRating,
				RecommendationRate: matched.recommendationRate,
				ReviewCount:        matched.reviewCount,
			}
			bulk = append(bulk, system.UpsertDoc{
				Filter: bson.M{"company_id": c.CompanyID},
				Doc:    c,
			})
			logrus.Infof("match %q — rating=%.1f recommendation=%s reviews=%d",
				c.CompanyName, matched.overallRating, matched.recommendationRate, matched.reviewCount)
		}

		if len(bulk) > 0 {
			if err := system.GetStorage().SetMany(ctx, constants.MongoCompaniesCollection, bulk); err != nil {
				logrus.Errorf("bulk write page %d: %v", page, err)
			} else {
				updated += len(bulk)
			}
		}

		logrus.Infof("page %d — %d matched, %d skipped (running total: updated=%d)",
			page, len(bulk), skipped, updated)

		if int64(page*pageSize) >= total {
			break
		}
		page++
	}

	logrus.Infof("done — %d companies updated, %d had no match", updated, skipped)
}

// parseHTML extracts all company entries from the #recruiter-company-list div.
func parseHTML(path string) ([]entry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	doc, err := goquery.NewDocumentFromReader(f)
	if err != nil {
		return nil, err
	}

	var entries []entry
	doc.Find("#recruiter-company-list .col").Each(func(_ int, col *goquery.Selection) {
		name := strings.TrimSpace(col.Find("h3").First().Text())
		if name == "" {
			return
		}

		// "55%" — text content of the progress bar inside .average-recommendation
		recommendationStr := strings.TrimSpace(col.Find(".average-recommendation .progress-bar").Text())

		// "2.7" — numeric rating next to the stars
		ratingStr := strings.TrimSpace(col.Find(".ms-2.strong").Text())
		rating, _ := strconv.ParseFloat(ratingStr, 64)

		// "40 Bewertungen" — find the .small div that contains "Bewertungen"
		reviewCount := 0
		col.Find(".small").Each(func(_ int, s *goquery.Selection) {
			text := strings.TrimSpace(s.Text())
			if strings.Contains(text, "Bewertungen") {
				parts := strings.Fields(text)
				if len(parts) > 0 {
					reviewCount, _ = strconv.Atoi(parts[0])
				}
			}
		})

		entries = append(entries, entry{
			companyName:        name,
			overallRating:      rating,
			recommendationRate: recommendationStr,
			reviewCount:        reviewCount,
		})
	})

	return entries, nil
}

// findMatch looks for a MongoDB company name in the HTML lookup.
// Tries exact match first, then substring containment in both directions.
func findMatch(normalizedMongoName string, lookup map[string]entry) (entry, bool) {
	// Exact match
	if e, ok := lookup[normalizedMongoName]; ok {
		return e, true
	}
	// Substring match
	for htmlName, e := range lookup {
		if strings.Contains(normalizedMongoName, htmlName) || strings.Contains(htmlName, normalizedMongoName) {
			return e, true
		}
	}
	return entry{}, false
}

// normalizeName lowercases and strips extra whitespace for fuzzy comparison.
func normalizeName(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

// resolveHTMLPath returns the path to recruiters.html sitting next to this
// source file (works with both `go run .` and compiled binary).
func resolveHTMLPath() string {
	// Prefer a file next to the running executable.
	if exe, err := os.Executable(); err == nil {
		p := filepath.Join(filepath.Dir(exe), "recruiters.html")
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	// Fall back to the source-file directory (go run).
	_, src, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(src), "recruiters.html")
}
