package main

import (
	"strings"
)

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
