package crawler

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/Saul-Punybz/folio/internal/models"
)

// BuildDomainMap returns a map of domain -> CrawlDomain for fast lookup.
func BuildDomainMap(ctx context.Context, domains *models.CrawlDomainStore) (map[string]*models.CrawlDomain, error) {
	list, err := domains.ListActive(ctx)
	if err != nil {
		return nil, fmt.Errorf("build domain map: %w", err)
	}
	m := make(map[string]*models.CrawlDomain, len(list))
	for i := range list {
		m[list[i].Domain] = &list[i]
	}
	return m, nil
}

// IsAllowedDomain checks if a URL belongs to an allowed crawl domain and
// returns the domain ID if so.
func IsAllowedDomain(rawURL string, domainMap map[string]*models.CrawlDomain) (uuid.UUID, bool) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return uuid.Nil, false
	}
	host := strings.ToLower(parsed.Hostname())

	if d, ok := domainMap[host]; ok {
		return d.ID, true
	}

	// Try with www prefix
	if !strings.HasPrefix(host, "www.") {
		if d, ok := domainMap["www."+host]; ok {
			return d.ID, true
		}
	} else {
		if d, ok := domainMap[strings.TrimPrefix(host, "www.")]; ok {
			return d.ID, true
		}
	}

	return uuid.Nil, false
}

// ScheduleNextCrawl calculates the next crawl time based on recrawl_hours.
func ScheduleNextCrawl(recrawlHours int) time.Time {
	if recrawlHours <= 0 {
		recrawlHours = 168 // Default: weekly
	}
	return time.Now().Add(time.Duration(recrawlHours) * time.Hour)
}

// DomainFromURL extracts the hostname from a URL.
func DomainFromURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return strings.ToLower(parsed.Hostname())
}
