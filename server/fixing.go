package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"strings"
	"time"

	ics "github.com/arran4/golang-ical"
)

// FixLog tracks which fixes have been applied to an iCal file
type FixLog struct {
	Fixes []string
}

// AddFix records a fix that was applied
func (fl *FixLog) AddFix(fix string) {
	fl.Fixes = append(fl.Fixes, fix)
	log.Printf("Applied fix: %s", fix)
}

// GetSummary returns a summary of all fixes applied
func (fl *FixLog) GetSummary() string {
	if len(fl.Fixes) == 0 {
		return "No fixes applied"
	}
	return fmt.Sprintf("Applied %d fixes: %s", len(fl.Fixes), strings.Join(fl.Fixes, ", "))
}

// Comprehensive calendar fixing function that addresses common RFC 5545 compliance issues
func fixCalendar(calendar *ics.Calendar) *FixLog {
	fixLog := &FixLog{}

	// Fix calendar-level properties
	fixCalendarProperties(calendar, fixLog)

	// Fix all events
	for i, event := range calendar.Events() {
		eventFixes := fixEvent(event)
		if len(eventFixes.Fixes) > 0 {
			fixLog.AddFix(fmt.Sprintf("Event %d: %s", i+1, strings.Join(eventFixes.Fixes, ", ")))
		}
	}

	// Fix all todos
	for i, todo := range calendar.Todos() {
		todoFixes := fixTodo(todo)
		if len(todoFixes.Fixes) > 0 {
			fixLog.AddFix(fmt.Sprintf("Todo %d: %s", i+1, strings.Join(todoFixes.Fixes, ", ")))
		}
	}

	return fixLog
}

func fixCalendarProperties(calendar *ics.Calendar, fixLog *FixLog) {
	// Helper function to get calendar property value
	getCalendarProperty := func(propertyName string) string {
		for _, prop := range calendar.CalendarProperties {
			if prop.IANAToken == propertyName {
				return prop.Value
			}
		}
		return ""
	}

	// Ensure VERSION is set correctly
	if getCalendarProperty("VERSION") != "2.0" {
		calendar.SetVersion("2.0")
		fixLog.AddFix("Set VERSION to 2.0")
	}

	// Ensure PRODID exists (RFC 5545: required property)
	// Only set our own if missing entirely - preserve existing valid PRODID
	if getCalendarProperty("PRODID") == "" {
		calendar.SetProductId("-//iCal Proxy Server//EN")
		fixLog.AddFix("Added missing PRODID")
	}

	// Set CALSCALE if not present or invalid (RFC 5545: default is GREGORIAN, only GREGORIAN is widely supported)
	calscale := getCalendarProperty("CALSCALE")
	if calscale == "" {
		calendar.SetCalscale("GREGORIAN")
		fixLog.AddFix("Added missing CALSCALE (GREGORIAN)")
	} else if calscale != "GREGORIAN" {
		// RFC 5545 allows other calendar scales, but GREGORIAN is the only widely supported one
		calendar.SetCalscale("GREGORIAN")
		fixLog.AddFix(fmt.Sprintf("Changed unsupported CALSCALE '%s' to GREGORIAN", calscale))
	}
}

func fixEvent(event *ics.VEvent) *FixLog {
	fixLog := &FixLog{}

	// Fix required properties
	fixRequiredEventProperties(event, fixLog)

	// Fix date-time properties
	fixEventDateTimes(event, fixLog)

	// Fix optional but commonly expected properties
	fixEventOptionalProperties(event, fixLog)

	// Fix nested components (alarms)
	fixEventAlarms(event, fixLog)

	return fixLog
}

func fixRequiredEventProperties(event *ics.VEvent, fixLog *FixLog) {
	// Ensure UID exists
	if event.GetProperty(ics.ComponentPropertyUniqueId) == nil {
		uid := generateUID()
		event.SetProperty(ics.ComponentPropertyUniqueId, uid)
		fixLog.AddFix("Generated missing UID")
	}

	// Ensure DTSTAMP exists
	if event.GetProperty(ics.ComponentPropertyDtstamp) == nil {
		now := time.Now().UTC().Format("20060102T150405Z")
		event.SetProperty(ics.ComponentPropertyDtstamp, now)
		fixLog.AddFix("Added missing DTSTAMP")
	}

	// Ensure SUMMARY exists (required for display)
	if event.GetProperty(ics.ComponentPropertySummary) == nil {
		event.SetProperty(ics.ComponentPropertySummary, "Event")
		fixLog.AddFix("Added default SUMMARY")
	}
}

func fixEventDateTimes(event *ics.VEvent, fixLog *FixLog) {
	dtstart := event.GetProperty(ics.ComponentPropertyDtStart)
	dtend := event.GetProperty(ics.ComponentPropertyDtEnd)

	// Ensure DTSTART exists
	if dtstart == nil {
		// Create a default start time (now)
		now := time.Now().UTC().Format("20060102T150405Z")
		event.SetProperty(ics.ComponentPropertyDtStart, now)
		dtstart = event.GetProperty(ics.ComponentPropertyDtStart)
		fixLog.AddFix("Added missing DTSTART")
	}

	// Fix DTSTART format
	if dtstart != nil {
		originalValue := dtstart.Value
		dtstart.Value = normalizeDateTime(dtstart.Value)
		if originalValue != dtstart.Value {
			fixLog.AddFix("Normalized DTSTART format")
		}
	}

	// Ensure DTEND exists and is after DTSTART
	if dtend == nil {
		// Create DTEND 1 hour after DTSTART
		if dtstart != nil {
			startTime, err := parseDateTime(dtstart.Value)
			if err == nil {
				endTime := startTime.Add(time.Hour)
				event.SetProperty(ics.ComponentPropertyDtEnd, endTime.UTC().Format("20060102T150405Z"))
			} else {
				// Fallback: use current time + 1 hour
				endTime := time.Now().Add(time.Hour).UTC().Format("20060102T150405Z")
				event.SetProperty(ics.ComponentPropertyDtEnd, endTime)
			}
		}
		dtend = event.GetProperty(ics.ComponentPropertyDtEnd)
		fixLog.AddFix("Added missing DTEND")
	}

	// Fix DTEND format
	if dtend != nil {
		originalValue := dtend.Value
		dtend.Value = normalizeDateTime(dtend.Value)
		if originalValue != dtend.Value {
			fixLog.AddFix("Normalized DTEND format")
		}
	}

	// Ensure DTEND is after DTSTART
	if dtstart != nil && dtend != nil {
		startTime, startErr := parseDateTime(dtstart.Value)
		endTime, endErr := parseDateTime(dtend.Value)

		if startErr == nil && endErr == nil && !endTime.After(startTime) {
			// Fix by adding 1 hour to start time
			newEndTime := startTime.Add(time.Hour)
			dtend.Value = newEndTime.UTC().Format("20060102T150405Z")
			fixLog.AddFix("Fixed DTEND to be after DTSTART")
		}
	}
}

func fixEventOptionalProperties(event *ics.VEvent, fixLog *FixLog) {
	// Add CREATED timestamp if missing
	if event.GetProperty(ics.ComponentPropertyCreated) == nil {
		now := time.Now().UTC().Format("20060102T150405Z")
		event.SetProperty(ics.ComponentPropertyCreated, now)
		fixLog.AddFix("Added missing CREATED timestamp")
	}

	// Add LAST-MODIFIED timestamp if missing
	if event.GetProperty(ics.ComponentPropertyLastModified) == nil {
		now := time.Now().UTC().Format("20060102T150405Z")
		event.SetProperty(ics.ComponentPropertyLastModified, now)
		fixLog.AddFix("Added missing LAST-MODIFIED timestamp")
	}

	// Validate and fix CLASS property (RFC 5545: "PUBLIC" / "PRIVATE" / "CONFIDENTIAL" / iana-token / x-name)
	class := event.GetProperty(ics.ComponentPropertyClass)
	if class == nil {
		event.SetProperty(ics.ComponentPropertyClass, "PUBLIC")
		fixLog.AddFix("Added missing CLASS (PUBLIC)")
	} else if class.Value != "" && !isValidClassValue(class.Value) {
		fixLog.AddFix(fmt.Sprintf("Invalid CLASS value '%s', changed to PUBLIC", class.Value))
		class.Value = "PUBLIC"
	}

	// Validate and fix STATUS property (RFC 5545: "TENTATIVE" / "CONFIRMED" / "CANCELLED" / iana-token / x-name)
	status := event.GetProperty(ics.ComponentPropertyStatus)
	if status == nil {
		event.SetProperty(ics.ComponentPropertyStatus, "CONFIRMED")
		fixLog.AddFix("Added missing STATUS (CONFIRMED)")
	} else if status.Value == "" {
		status.Value = "CONFIRMED"
		fixLog.AddFix("Set empty STATUS to CONFIRMED")
	} else if !isValidStatusValue(status.Value) {
		fixLog.AddFix(fmt.Sprintf("Invalid STATUS value '%s', changed to CONFIRMED", status.Value))
		status.Value = "CONFIRMED"
	}

	// Validate and fix TRANSP property (RFC 5545: "OPAQUE" / "TRANSPARENT" / iana-token / x-name)
	transp := event.GetProperty(ics.ComponentPropertyTransp)
	if transp == nil {
		event.SetProperty(ics.ComponentPropertyTransp, "OPAQUE")
		fixLog.AddFix("Added missing TRANSP (OPAQUE)")
	} else if transp.Value == "" {
		transp.Value = "OPAQUE"
		fixLog.AddFix("Set empty TRANSP to OPAQUE")
	} else if !isValidTranspValue(transp.Value) {
		fixLog.AddFix(fmt.Sprintf("Invalid TRANSP value '%s', changed to OPAQUE", transp.Value))
		transp.Value = "OPAQUE"
	}
}

func fixEventAlarms(event *ics.VEvent, fixLog *FixLog) {
	// Fix existing alarms
	alarmCount := 0
	for _, alarm := range event.Alarms() {
		alarmCount++

		// Validate and fix ACTION property (RFC 5545: required, "AUDIO" / "DISPLAY" / "EMAIL" / iana-token / x-name)
		action := alarm.GetProperty(ics.ComponentPropertyAction)
		if action == nil {
			alarm.SetProperty(ics.ComponentPropertyAction, "DISPLAY")
			fixLog.AddFix(fmt.Sprintf("Added missing ACTION to alarm %d", alarmCount))
		} else if action.Value == "" {
			action.Value = "DISPLAY"
			fixLog.AddFix(fmt.Sprintf("Set empty ACTION to DISPLAY in alarm %d", alarmCount))
		} else if !isValidActionValue(action.Value) {
			fixLog.AddFix(fmt.Sprintf("Invalid ACTION value '%s' in alarm %d, changed to DISPLAY", action.Value, alarmCount))
			action.Value = "DISPLAY"
		}

		// Ensure TRIGGER property exists (RFC 5545: required)
		if alarm.GetProperty(ics.ComponentPropertyTrigger) == nil {
			alarm.SetProperty(ics.ComponentPropertyTrigger, "-PT15M") // 15 minutes before
			fixLog.AddFix(fmt.Sprintf("Added missing TRIGGER to alarm %d", alarmCount))
		}

		// Ensure DESCRIPTION exists for DISPLAY and EMAIL actions (RFC 5545: required for these actions)
		actionValue := ""
		if alarm.GetProperty(ics.ComponentPropertyAction) != nil {
			actionValue = strings.ToUpper(alarm.GetProperty(ics.ComponentPropertyAction).Value)
		}

		if (actionValue == "DISPLAY" || actionValue == "EMAIL") &&
			alarm.GetProperty(ics.ComponentPropertyDescription) == nil {
			summary := event.GetProperty(ics.ComponentPropertySummary)
			if summary != nil {
				alarm.SetProperty(ics.ComponentPropertyDescription, summary.Value)
			} else {
				alarm.SetProperty(ics.ComponentPropertyDescription, "Event Reminder")
			}
			fixLog.AddFix(fmt.Sprintf("Added missing DESCRIPTION to %s alarm %d", actionValue, alarmCount))
		}

		// Ensure SUMMARY exists for EMAIL actions (RFC 5545: required for EMAIL)
		if actionValue == "EMAIL" && alarm.GetProperty(ics.ComponentPropertySummary) == nil {
			summary := event.GetProperty(ics.ComponentPropertySummary)
			if summary != nil {
				alarm.SetProperty(ics.ComponentPropertySummary, summary.Value)
			} else {
				alarm.SetProperty(ics.ComponentPropertySummary, "Event Reminder")
			}
			fixLog.AddFix(fmt.Sprintf("Added missing SUMMARY to EMAIL alarm %d", alarmCount))
		}
	}
}

func fixTodo(todo *ics.VTodo) *FixLog {
	fixLog := &FixLog{}

	// Ensure UID exists
	if todo.GetProperty(ics.ComponentPropertyUniqueId) == nil {
		uid := generateUID()
		todo.SetProperty(ics.ComponentPropertyUniqueId, uid)
		fixLog.AddFix("Generated missing UID for TODO")
	}

	// Ensure DTSTAMP exists
	if todo.GetProperty(ics.ComponentPropertyDtstamp) == nil {
		now := time.Now().UTC().Format("20060102T150405Z")
		todo.SetProperty(ics.ComponentPropertyDtstamp, now)
		fixLog.AddFix("Added missing DTSTAMP to TODO")
	}

	// Ensure SUMMARY exists
	if todo.GetProperty(ics.ComponentPropertySummary) == nil {
		todo.SetProperty(ics.ComponentPropertySummary, "Task")
		fixLog.AddFix("Added default SUMMARY to TODO")
	}

	return fixLog
}

func generateUID() string {
	// Generate a random UID
	bytes := make([]byte, 16)
	rand.Read(bytes)
	return hex.EncodeToString(bytes) + "@ical-proxy.local"
}

func normalizeDateTime(value string) string {
	// Remove any invalid characters and normalize format
	cleaned := strings.ReplaceAll(value, " ", "")
	cleaned = strings.ReplaceAll(cleaned, "-", "")
	cleaned = strings.ReplaceAll(cleaned, ":", "")

	// If it looks like a date-time but doesn't end with Z, add it
	if len(cleaned) == 15 && !strings.HasSuffix(cleaned, "Z") {
		cleaned += "Z"
	}

	// If it's too short, pad with default time
	if len(cleaned) == 8 {
		cleaned += "T000000Z"
	}

	return cleaned
}

func parseDateTime(value string) (time.Time, error) {
	// Try different formats
	formats := []string{
		"20060102T150405Z",
		"20060102T150405",
		"20060102",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, value); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("invalid date format: %s", value)
}

func applyPostSerializationFixes(icalData string, fixLog *FixLog) string {
	// Fix TZID parameters on UTC times
	// RFC 5545: TZID parameter MUST NOT be applied to DATE-TIME properties whose time values are specified in UTC
	fixed := fixTzidOnUtcTimes(icalData)
	if fixed != icalData {
		fixLog.AddFix("Removed TZID parameters from UTC times")
	}
	return fixed
}

func fixTzidOnUtcTimes(icalData string) string {
	// Fix TZID parameters on UTC times more robustly
	// RFC 5545: TZID parameter MUST NOT be applied to DATE-TIME properties whose time values are specified in UTC
	lines := strings.Split(icalData, "\r\n")

	for i, line := range lines {
		// Check if line contains DTSTART or DTEND with TZID parameter
		if (strings.HasPrefix(line, "DTSTART;") || strings.HasPrefix(line, "DTEND;")) &&
			strings.Contains(line, "TZID=") {

			// Find the colon that separates property from value
			colonIndex := strings.Index(line, ":")
			if colonIndex != -1 {
				value := line[colonIndex+1:]

				// Check if the value ends with Z (UTC indicator)
				if strings.HasSuffix(value, "Z") {
					// Extract just the property name (DTSTART or DTEND)
					propertyName := "DTSTART"
					if strings.HasPrefix(line, "DTEND;") {
						propertyName = "DTEND"
					}

					// Reconstruct line without TZID parameter
					lines[i] = propertyName + ":" + value
				}
			}
		}
	}

	return strings.Join(lines, "\r\n")
}
