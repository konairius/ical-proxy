package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"

	ics "github.com/arran4/golang-ical"
)

func main() {
	http.HandleFunc("/fix-ical", handleFixIcal)
	port := ":8080"
	fmt.Printf("Starting server on port %s\n", port)
	http.ListenAndServe(port, nil)
}

func handleFixIcal(w http.ResponseWriter, r *http.Request) {
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

	resp, err := http.Get(urlParam)
	if err != nil || resp.StatusCode != http.StatusOK {
		http.Error(w, "Failed to fetch iCal file", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	icalData, err := io.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, "Failed to read iCal file content", http.StatusInternalServerError)
		return
	}

	fixedICal, err := FixICalData(icalData)
	if err != nil {
		http.Error(w, "Failed to fix iCal format: "+err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "text/calendar")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fixedICal))
}

// FixICalData takes raw iCal data and returns a fixed, RFC 5545 compliant version
func FixICalData(icalData []byte) (string, error) {
	if len(icalData) == 0 {
		return "", fmt.Errorf("empty iCal data")
	}

	log.Printf("Starting iCal fixing process for %d bytes of data", len(icalData))

	calendar, err := ics.ParseCalendar(bytes.NewReader(icalData))
	if err != nil {
		return "", fmt.Errorf("invalid iCal format: %w", err)
	}

	// Apply comprehensive fixes to ensure RFC 5545 compliance
	fixLog := fixCalendar(calendar)

	// Serialize with proper CRLF line endings (RFC 5545 requirement)
	fixedICal := calendar.Serialize(ics.WithNewLine("\r\n"))

	// Apply post-serialization fixes for issues that can't be handled during object manipulation
	fixedICal = applyPostSerializationFixes(fixedICal, fixLog)

	// Log summary of fixes applied
	log.Printf("iCal fixing complete. %s", fixLog.GetSummary())

	return fixedICal, nil
}
