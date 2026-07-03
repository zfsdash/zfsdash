package zfs

// parse.go - additional parsing helpers
// Core parsing is in local.go alongside the LocalCollector
// This file is reserved for shared parsing utilities

import "strings"

// NormalizeHealth maps various ZFS health strings to standard values
func NormalizeHealth(h string) string {
	switch strings.ToUpper(h) {
	case "ONLINE":
		return "ONLINE"
	case "DEGRADED":
		return "DEGRADED"
	case "FAULTED":
		return "FAULTED"
	case "OFFLINE":
		return "OFFLINE"
	case "REMOVED":
		return "REMOVED"
	case "UNAVAIL":
		return "UNAVAIL"
	default:
		return h
	}
}
