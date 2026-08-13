package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	ics "github.com/arran4/golang-ical"
)

func main() {
	http.HandleFunc("/proxy", handleProxy)
	http.HandleFunc("/health", handleHealth)

	// PORT is validated into an int rather than used as a string. It came
	// straight from the environment into both the listen address and a log
	// line, which gosec flags as log injection (G706): anything in PORT is
	// reproduced verbatim in the log, newlines included, so a value like
	// "8080\nFATAL forged entry" writes a second line of its own.
	//
	// Parsing it settles that at the source. A port is a number, an
	// unparseable one is a misconfiguration worth failing on rather than
	// quietly ignoring, and once it is an int there is nothing left to
	// inject — the log below formats %d.
	port := 8080
	if raw := os.Getenv("PORT"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > 65535 {
			log.Fatalf("Invalid PORT: expected an integer between 1 and 65535")
		}
		port = parsed
	}

	// Create server with timeouts to address gosec G114
	server := &http.Server{
		Addr:           fmt.Sprintf(":%d", port),
		Handler:        nil,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		IdleTimeout:    15 * time.Second,
		MaxHeaderBytes: 1 << 20, // 1 MB
	}

	fmt.Printf("Starting server on port %d\n", port)
	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("Failed to start server on port %d: %v", port, err)
	}
}

func handleProxy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	urlParam := r.URL.Query().Get("url")
	if urlParam == "" {
		http.Error(w, "Missing 'url' parameter", http.StatusBadRequest)
		return
	}

	parsedURL, err := url.Parse(urlParam)
	if err != nil || !parsedURL.IsAbs() {
		http.Error(w, "Invalid 'url' parameter", http.StatusBadRequest)
		return
	}

	// Restrict the scheme. Absolute is not enough: url.Parse happily accepts
	// file:///etc/passwd and ftp://, and net/http would follow whatever it
	// has a transport for. A calendar feed is fetched over HTTP or HTTPS and
	// nothing else.
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		http.Error(w, "Invalid 'url' parameter: only http and https are supported", http.StatusBadRequest)
		return
	}

	// Parse optional SUMMARY rewriting parameters
	rewrite, err := parseSummaryRewrite(r.URL.Query())
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Parse optional date filtering parameters
	fromParam := r.URL.Query().Get("from")
	toParam := r.URL.Query().Get("to")

	var fromDate, toDate *time.Time

	if fromParam != "" {
		parsed, err := time.Parse("2006-01-02", fromParam)
		if err != nil {
			http.Error(w, "Invalid 'from' date format. Use YYYY-MM-DD", http.StatusBadRequest)
			return
		}
		fromDate = &parsed
	}

	if toParam != "" {
		parsed, err := time.Parse("2006-01-02", toParam)
		if err != nil {
			http.Error(w, "Invalid 'to' date format. Use YYYY-MM-DD", http.StatusBadRequest)
			return
		}
		toDate = &parsed
	}

	// Use http.Client with timeout to address gosec G107
	client := &http.Client{
		Timeout: 30 * time.Second,
	}
	// #nosec G704 -- fetching a caller-supplied URL is what this service is
	// for; a proxy that only fetched a fixed address would not be a proxy.
	// The request is constrained to what that job needs: the URL must be
	// absolute and http/https (checked above), the client has a 30s timeout,
	// and the response is parsed as iCalendar rather than returned raw.
	// Anything further — an upstream allowlist, or refusing private address
	// ranges — is a deployment decision, and this one runs behind a
	// CiliumNetworkPolicy that already bounds where it may connect.
	resp, err := client.Get(urlParam)
	if err != nil || resp.StatusCode != http.StatusOK {
		http.Error(w, "Failed to fetch iCal file", http.StatusInternalServerError)
		return
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			log.Printf("Error closing response body: %v", closeErr)
		}
	}()

	icalData, err := io.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, "Failed to read iCal file content", http.StatusInternalServerError)
		return
	}

	fixedICal, err := ProcessICalData(icalData, fromDate, toDate, rewrite)
	if err != nil {
		http.Error(w, "Failed to process iCal data: "+err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "text/calendar")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte(fixedICal)); err != nil {
		log.Printf("Failed to write response: %v", err)
	}
}

// SummaryRewrite is an optional substitution applied to every VEVENT SUMMARY.
//
// Some upstream feeds append the name of the whole calendar to every single
// event, so a bin collection arrives as
//
//	Altpapier| Abfuhrkalender - Landkreis Amberg-Sulzbach
//
// which is one useful word followed by fifty characters that are identical on
// every event in the feed. Consumers cannot fix this: the text is what the
// publisher put in SUMMARY, and calendar clients render it verbatim.
type SummaryRewrite struct {
	// Pattern is matched against the existing SUMMARY.
	Pattern *regexp.Regexp
	// Replacement may reference capture groups as $1, $2 and so on.
	Replacement string
}

// parseSummaryRewrite reads the summary_match and summary_replace parameters.
// Returns nil (and no error) when no rewriting was requested.
func parseSummaryRewrite(query url.Values) (*SummaryRewrite, error) {
	match := query.Get("summary_match")
	replace := query.Get("summary_replace")

	if match == "" {
		if replace != "" {
			return nil, fmt.Errorf("'summary_replace' requires 'summary_match'")
		}
		return nil, nil
	}

	// Bounded so a pathological pattern cannot be handed to the compiler.
	// Matching itself is safe regardless: Go's regexp is RE2, which runs in
	// time linear in the input and has no catastrophic backtracking.
	const maxPatternLen = 512
	if len(match) > maxPatternLen {
		return nil, fmt.Errorf("'summary_match' must be at most %d characters", maxPatternLen)
	}

	pattern, err := regexp.Compile(match)
	if err != nil {
		return nil, fmt.Errorf("invalid 'summary_match' expression: %w", err)
	}

	return &SummaryRewrite{Pattern: pattern, Replacement: replace}, nil
}

// rewriteSummaries applies the substitution to every event's SUMMARY and
// returns the number of events changed.
//
// An event whose rewritten SUMMARY would be empty is left alone. Trading a
// too-long title for no title at all is not an improvement, and it would strip
// a property that RFC 5545 expects to carry the event's display text.
func rewriteSummaries(calendar *ics.Calendar, rewrite *SummaryRewrite) int {
	if rewrite == nil {
		return 0
	}

	changed := 0
	for _, event := range calendar.Events() {
		prop := event.GetProperty(ics.ComponentPropertySummary)
		if prop == nil {
			continue
		}

		original := prop.Value
		rewritten := strings.TrimSpace(rewrite.Pattern.ReplaceAllString(original, rewrite.Replacement))
		if rewritten == "" || rewritten == original {
			continue
		}

		event.SetProperty(ics.ComponentPropertySummary, rewritten)
		changed++
	}

	return changed
}

// ProcessICalData takes raw iCal data and returns a processed version with
// optional date filtering and optional SUMMARY rewriting
func ProcessICalData(icalData []byte, fromDate, toDate *time.Time, rewrite *SummaryRewrite) (string, error) {
	if len(icalData) == 0 {
		return "", fmt.Errorf("empty iCal data")
	}

	log.Printf("Starting iCal processing for %d bytes of data", len(icalData))

	calendar, err := ics.ParseCalendar(bytes.NewReader(icalData))
	if err != nil {
		return "", fmt.Errorf("invalid iCal format: %w", err)
	}

	// Apply date filtering if specified
	if fromDate != nil || toDate != nil {
		filterEventsByDate(calendar, fromDate, toDate)
	}

	// Rewrite summaries before the compliance pass, so the fixer sees the
	// text that will actually be served.
	if changed := rewriteSummaries(calendar, rewrite); changed > 0 {
		log.Printf("Rewrote SUMMARY on %d event(s)", changed)
	}

	// Apply comprehensive fixes to ensure RFC 5545 compliance
	fixLog := fixCalendar(calendar)

	// Serialize with proper CRLF line endings (RFC 5545 requirement)
	fixedICal := calendar.Serialize(ics.WithNewLine("\r\n"))

	// Apply post-serialization fixes for issues that can't be handled during object manipulation
	fixedICal = applyPostSerializationFixes(fixedICal, fixLog)

	// Log summary of fixes applied
	log.Printf("iCal processing complete. %s", fixLog.GetSummary())

	return fixedICal, nil
}

// filterEventsByDate removes events outside the specified date range
func filterEventsByDate(calendar *ics.Calendar, fromDate, toDate *time.Time) {
	events := calendar.Events()
	eventsToRemove := []*ics.VEvent{}

	for _, event := range events {
		shouldRemove := false

		// Get event start time
		startProp := event.GetProperty(ics.ComponentPropertyDtStart)
		if startProp != nil {
			if eventStart, err := parseEventDate(startProp.Value); err == nil {
				// Check if event is before fromDate
				if fromDate != nil && eventStart.Before(*fromDate) {
					shouldRemove = true
				}

				// Check if event is after toDate
				if toDate != nil && eventStart.After(toDate.AddDate(0, 0, 1)) { // Add 1 day to include events on toDate
					shouldRemove = true
				}
			}
		}

		if shouldRemove {
			eventsToRemove = append(eventsToRemove, event)
		}
	}

	// Remove filtered events
	for _, event := range eventsToRemove {
		calendar.RemoveEvent(event.Id())
	}

	log.Printf("Filtered out %d events based on date range", len(eventsToRemove))
}

// parseEventDate parses various iCal date formats
func parseEventDate(dateStr string) (time.Time, error) {
	// Try different date formats used in iCal
	formats := []string{
		"20060102T150405Z",     // UTC format
		"20060102T150405",      // Local format
		"20060102",             // Date only
		"2006-01-02T15:04:05Z", // RFC3339 UTC
		"2006-01-02T15:04:05",  // RFC3339 local
		"2006-01-02",           // Date only with dashes
	}

	for _, format := range formats {
		if t, err := time.Parse(format, dateStr); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("unable to parse date: %s", dateStr)
}

// FixICalData is kept for backward compatibility but now uses ProcessICalData
func FixICalData(icalData []byte) (string, error) {
	return ProcessICalData(icalData, nil, nil, nil)
}

// handleHealth provides a simple health check endpoint
func handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte(`{"status":"healthy","service":"ical-proxy"}`)); err != nil {
		log.Printf("Failed to write health response: %v", err)
	}
}
