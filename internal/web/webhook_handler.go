package web

import (
	"io"
	"log/slog"
	"net/http"
)

// HandleStripeWebhook handles POST /api/webhooks/stripe
// Receives Stripe webhook events. Full verification added in v0.2 with Stripe key.
func (h *Handler) HandleStripeWebhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}
	_ = body
	slog.Info("stripe webhook received", "content-length", r.ContentLength)
	w.WriteHeader(http.StatusOK)
}
