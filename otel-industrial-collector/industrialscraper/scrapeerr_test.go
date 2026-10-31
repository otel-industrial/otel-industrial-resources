package industrialscraper

import (
	"errors"
	"strings"
	"testing"

	"go.opentelemetry.io/collector/scraper/scrapererror"
)

func TestFailuresNoneIsNil(t *testing.T) {
	var f Failures
	if err := f.Err(); err != nil {
		t.Fatalf("Err() = %v, want nil when nothing failed", err)
	}
}

func TestFailuresCountsFailedAndSkipped(t *testing.T) {
	readErr := errors.New("tag 1: bad address")
	stopErr := errors.New("transport: EOF")
	var f Failures
	f.Add(readErr)
	f.AddN(3, stopErr) // the failing item plus two never read
	err := f.Err()

	var partial scrapererror.PartialScrapeError
	if !errors.As(err, &partial) {
		t.Fatalf("Err() = %T, want scrapererror.PartialScrapeError", err)
	}
	if partial.Failed != 4 {
		t.Errorf("Failed = %d, want 4", partial.Failed)
	}
	// PartialScrapeError has no Unwrap, so the causes surface in the message
	// the controller logs.
	if msg := err.Error(); !strings.Contains(msg, readErr.Error()) || !strings.Contains(msg, stopErr.Error()) {
		t.Errorf("Err() = %q, want both causes in the message", msg)
	}
}
