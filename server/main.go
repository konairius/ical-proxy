package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"

	ics "github.com/arran4/golang-ical"
)

func main() {
	http.HandleFunc("/proxy", handleProxy)
	http.HandleFunc("/health", handleHealth)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Create server with timeouts to address gosec G114
	server := &http.Server{
		Addr:           ":" + port,
		Handler:        nil,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		IdleTimeout:    15 * time.Second,
		MaxHeaderBytes: 1 << 20, // 1 MB
	}

	fmt.Printf("Starting server on port %s\n", port)
	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("Failed to start server on port %s: %v", port, err)
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

	fixedICal, err := ProcessICalData(icalData, fromDate, toDate)
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

// ProcessICalData takes raw iCal data and returns a processed version with optional date filtering
func ProcessICalData(icalData []byte, fromDate, toDate *time.Time) (string, error) {
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
	return ProcessICalData(icalData, nil, nil)
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
