package web

import (
	"encoding/json"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

type ChecksumAuditResult struct {
	Pool        string    `json:"pool"`
	Timestamp   time.Time `json:"timestamp"`
	CksumErrors int64     `json:"cksum_errors"`
	Status      string    `json:"status"`
	Remediation string    `json:"remediation,omitempty"`
}

func (h *Handler) handleChecksumAudit(w http.ResponseWriter, r *http.Request) {
	pool := chi.URLParam(r, "pool")
	result := ChecksumAuditResult{Pool: pool, Timestamp: time.Now()}
	out, err := exec.CommandContext(r.Context(), "zpool", "status", "-p", pool).Output()
	if err != nil {
		result.Status = "unavailable"
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
		return
	}
	newline := string([]byte{10})
	var totalCksum int64
	for _, line := range strings.Split(string(out), newline) {
		fields := strings.Fields(line)
		if len(fields) >= 5 {
			state := fields[1]
			if state == "ONLINE" || state == "DEGRADED" || state == "FAULTED" {
				if ce, err := strconv.ParseInt(fields[4], 10, 64); err == nil {
					totalCksum += ce
				}
			}
		}
	}
	result.CksumErrors = totalCksum
	if totalCksum > 0 {
		result.Status = "errors_detected"
		result.Remediation = "zpool scrub " + pool + " && zpool status -v " + pool
	} else {
		result.Status = "clean"
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (h *Handler) handleChecksumAuditAll(w http.ResponseWriter, r *http.Request) {
	out, err := exec.CommandContext(r.Context(), "zpool", "list", "-H", "-o", "name").Output()
	if err != nil {
		http.Error(w, "zpool list failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	pools := strings.Fields(string(out))
	var results []ChecksumAuditResult
	newline := string([]byte{10})
	for _, pool := range pools {
		statusOut, err := exec.CommandContext(r.Context(), "zpool", "status", "-p", pool).Output()
		if err != nil {
			continue
		}
		var totalCksum int64
		for _, line := range strings.Split(string(statusOut), newline) {
			fields := strings.Fields(line)
			if len(fields) >= 5 {
				state := fields[1]
				if state == "ONLINE" || state == "DEGRADED" || state == "FAULTED" {
					if ce, err := strconv.ParseInt(fields[4], 10, 64); err == nil {
						totalCksum += ce
					}
				}
			}
		}
		res := ChecksumAuditResult{Pool: pool, Timestamp: time.Now(), CksumErrors: totalCksum}
		if totalCksum > 0 {
			res.Status = "errors_detected"
			res.Remediation = "zpool scrub " + pool
		} else {
			res.Status = "clean"
		}
		results = append(results, res)
	}
	if results == nil {
		results = []ChecksumAuditResult{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"pools":     results,
		"audited":   len(results),
		"timestamp": time.Now(),
	})
}
