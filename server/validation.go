package main

import (
	"bytes"
	"strings"

	ics "github.com/arran4/golang-ical"
)

// Comprehensive validation for iCal data (used for logging/debugging, not blocking)
func isValidICal(data string) bool {
	// Basic structure checks
	if !bytes.Contains([]byte(data), []byte("BEGIN:VCALENDAR")) ||
		!bytes.Contains([]byte(data), []byte("END:VCALENDAR")) {
		return false
	}

	// Check for required calendar properties
	if !bytes.Contains([]byte(data), []byte("VERSION:2.0")) ||
		!bytes.Contains([]byte(data), []byte("PRODID:")) {
		return false
	}

	// Try to parse the calendar
	calendar, err := ics.ParseCalendar(bytes.NewReader([]byte(data)))
	if err != nil {
		return false
	}

	// Validate calendar structure
	if !validateCalendarStructure(calendar) {
		return false
	}

	return true
}

func validateCalendarStructure(calendar *ics.Calendar) bool {
	// Check for at least one component (event, todo, etc.)
	if len(calendar.Events()) == 0 && len(calendar.Todos()) == 0 {
		return false
	}

	// Validate each event
	for _, event := range calendar.Events() {
		if !validateEvent(event) {
			return false
		}
	}

	// Validate each todo
	for _, todo := range calendar.Todos() {
		if !validateTodo(todo) {
			return false
		}
	}

	return true
}

func validateEvent(event *ics.VEvent) bool {
	// Check required properties according to RFC 5545
	requiredProps := []ics.ComponentProperty{
		ics.ComponentPropertyUniqueId,
		ics.ComponentPropertyDtstamp,
	}

	for _, prop := range requiredProps {
		if event.GetProperty(prop) == nil {
			return false
		}
	}

	// Must have either DTEND or DURATION (we'll assume DTEND for simplicity)
	dtstart := event.GetProperty(ics.ComponentPropertyDtStart)
	dtend := event.GetProperty(ics.ComponentPropertyDtEnd)
	duration := event.GetProperty(ics.ComponentPropertyDuration)

	if dtstart == nil {
		return false
	}

	if dtend == nil && duration == nil {
		return false
	}

	// Validate date-time formats
	if !isValidDateTime(dtstart.Value) {
		return false
	}

	if dtend != nil && !isValidDateTime(dtend.Value) {
		return false
	}

	// Validate date-time logic
	if dtend != nil {
		startTime, startErr := parseDateTime(dtstart.Value)
		endTime, endErr := parseDateTime(dtend.Value)

		if startErr == nil && endErr == nil && !endTime.After(startTime) {
			return false
		}
	}

	// Validate alarms
	for _, alarm := range event.Alarms() {
		if !validateAlarm(alarm) {
			return false
		}
	}

	return true
}

func validateTodo(todo *ics.VTodo) bool {
	// Check required properties for TODO
	requiredProps := []ics.ComponentProperty{
		ics.ComponentPropertyUniqueId,
		ics.ComponentPropertyDtstamp,
	}

	for _, prop := range requiredProps {
		if todo.GetProperty(prop) == nil {
			return false
		}
	}

	return true
}

func validateAlarm(alarm *ics.VAlarm) bool {
	// ACTION is required
	action := alarm.GetProperty(ics.ComponentPropertyAction)
	if action == nil {
		return false
	}

	// TRIGGER is required
	trigger := alarm.GetProperty(ics.ComponentPropertyTrigger)
	if trigger == nil {
		return false
	}

	// Additional requirements based on ACTION type
	switch action.Value {
	case "DISPLAY":
		// DESCRIPTION is required for DISPLAY action
		if alarm.GetProperty(ics.ComponentPropertyDescription) == nil {
			return false
		}
	case "EMAIL":
		// DESCRIPTION and SUMMARY are required for EMAIL action
		if alarm.GetProperty(ics.ComponentPropertyDescription) == nil ||
			alarm.GetProperty(ics.ComponentPropertySummary) == nil {
			return false
		}
	}

	return true
}

// Enhanced helper function to validate date-time format
func isValidDateTime(value string) bool {
	// Check for UTC format ending with 'Z'
	if len(value) == 16 && value[15] == 'Z' {
		return true
	}
	// Check for local date-time format (e.g., YYYYMMDDTHHMMSS)
	if len(value) == 15 {
		return true
	}
	// Check for date-only format (e.g., YYYYMMDD)
	if len(value) == 8 {
		return true
	}
	return false
}

// RFC 5545 property value validation functions

// isValidClassValue validates CLASS property values according to RFC 5545
func isValidClassValue(value string) bool {
	// RFC 5545: classparam = "CLASS" "=" ("PUBLIC" / "PRIVATE" / "CONFIDENTIAL" / iana-token / x-name)
	standardValues := []string{"PUBLIC", "PRIVATE", "CONFIDENTIAL"}
	for _, valid := range standardValues {
		if strings.EqualFold(value, valid) {
			return true
		}
	}
	// Also allow IANA tokens and X-names (starting with X-)
	if strings.HasPrefix(strings.ToUpper(value), "X-") {
		return true
	}
	// For simplicity, we'll be conservative and only allow standard values for now
	return false
}

// isValidStatusValue validates STATUS property values according to RFC 5545
func isValidStatusValue(value string) bool {
	// RFC 5545: statvalue-event = "TENTATIVE" / "CONFIRMED" / "CANCELLED"
	standardValues := []string{"TENTATIVE", "CONFIRMED", "CANCELLED"}
	for _, valid := range standardValues {
		if strings.EqualFold(value, valid) {
			return true
		}
	}
	// Also allow IANA tokens and X-names
	if strings.HasPrefix(strings.ToUpper(value), "X-") {
		return true
	}
	return false
}

// isValidTranspValue validates TRANSP property values according to RFC 5545
func isValidTranspValue(value string) bool {
	// RFC 5545: transparam = "TRANSP" "=" ("OPAQUE" / "TRANSPARENT")
	standardValues := []string{"OPAQUE", "TRANSPARENT"}
	for _, valid := range standardValues {
		if strings.EqualFold(value, valid) {
			return true
		}
	}
	// Also allow IANA tokens and X-names
	if strings.HasPrefix(strings.ToUpper(value), "X-") {
		return true
	}
	return false
}

// isValidActionValue validates ACTION property values according to RFC 5545
func isValidActionValue(value string) bool {
	// RFC 5545: action = "AUDIO" / "DISPLAY" / "EMAIL" / iana-token / x-name
	standardValues := []string{"AUDIO", "DISPLAY", "EMAIL"}
	for _, valid := range standardValues {
		if strings.EqualFold(value, valid) {
			return true
		}
	}
	// Also allow IANA tokens and X-names
	if strings.HasPrefix(strings.ToUpper(value), "X-") {
		return true
	}
	return false
}
