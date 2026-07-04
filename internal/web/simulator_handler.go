package web

import (
	"encoding/json"
	"net/http"

	"github.com/zfsdash/zfsdash/internal/simulator"
)

// SimulatorHandler handles pool rebalance simulation requests.
type SimulatorHandler struct {
	sim *simulator.Simulator
}

// NewSimulatorHandler creates a new simulator handler.
func NewSimulatorHandler() *SimulatorHandler {
	return &SimulatorHandler{sim: simulator.NewSimulator()}
}

// SimulateRebalanceRequest is the JSON request body.
type SimulateRebalanceRequest struct {
	Pool     simulator.PoolConfig        `json:"pool"`
	Proposal simulator.RebalanceProposal `json:"proposal"`
}

// HandleSimulateRebalance handles POST /api/simulator/rebalance
func (h *SimulatorHandler) HandleSimulateRebalance(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req SimulateRebalanceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	if req.Pool.TotalSize == 0 {
		http.Error(w, "pool.total_size is required", http.StatusBadRequest)
		return
	}
	if len(req.Proposal.AddVdevs) == 0 {
		http.Error(w, "proposal.add_vdevs must contain at least one vdev", http.StatusBadRequest)
		return
	}

	result := h.sim.SimulateRebalance(req.Pool, req.Proposal)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}
