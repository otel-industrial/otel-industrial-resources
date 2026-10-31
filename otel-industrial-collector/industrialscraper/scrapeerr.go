package industrialscraper

import (
	"errors"

	"go.opentelemetry.io/collector/scraper/scrapererror"
)

// Failures counts the items a scrape could not read and builds the
// scrapererror the scraperhelper controller expects, so the values that were
// read are still emitted and the failures land in
// otelcol_scraper_errored_metric_points. See README.
type Failures struct {
	errs   []error
	failed int
}

// Add records one item that failed to read.
func (f *Failures) Add(err error) { f.AddN(1, err) }

// AddN records n items not read for one cause, e.g. the failing item plus the
// rest of the scan skipped after a transport error or the scrape deadline.
func (f *Failures) AddN(n int, err error) {
	f.errs = append(f.errs, err)
	f.failed += n
}

// Err returns a scrapererror.PartialScrapeError, or nil when nothing failed.
func (f *Failures) Err() error {
	if len(f.errs) == 0 {
		return nil
	}
	return scrapererror.NewPartialScrapeError(errors.Join(f.errs...), f.failed)
}
