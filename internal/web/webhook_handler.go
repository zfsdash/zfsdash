package web

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"

	"github.com/stripe/stripe-go/v78"
	"github.com/stripe/stripe-go/v78/webhook"

	"github.com/zfsdash/zfsdash/internal/db"
)

func StripeWebhookHandler(store *db.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "failed to read body", http.StatusBadRequest)
			return
		}
		sigHeader := r.Header.Get("Stripe-Signature")
		if sigHeader == "" {
			http.Error(w, "missing Stripe-Signature header", http.StatusBadRequest)
			return
		}
		webhookSecret := os.Getenv("STRIPE_WEBHOOK_SECRET")
		if webhookSecret == "" {
			slog.Error("STRIPE_WEBHOOK_SECRET not set")
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		event, err := webhook.ConstructEvent(body, sigHeader, webhookSecret)
		if err != nil {
			slog.Warn("webhook signature verification failed", "err", err)
			http.Error(w, "webhook signature verification failed", http.StatusBadRequest)
			return
		}
		switch event.Type {
		case "checkout.session.completed":
			if err := handleCheckoutCompleted(store, event); err != nil {
				slog.Error("checkout.session.completed failed", "err", err)
				http.Error(w, "internal server error", http.StatusInternalServerError)
				return
			}
		case "customer.subscription.deleted":
			if err := handleSubscriptionDeleted(store, event); err != nil {
				slog.Error("customer.subscription.deleted failed", "err", err)
				http.Error(w, "internal server error", http.StatusInternalServerError)
				return
			}
		case "invoice.payment_failed":
			if err := handlePaymentFailed(store, event); err != nil {
				slog.Error("invoice.payment_failed failed", "err", err)
				http.Error(w, "internal server error", http.StatusInternalServerError)
				return
			}
		default:
			slog.Debug("ignoring webhook event", "type", event.Type)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "received"})
	}
}

func handleCheckoutCompleted(store *db.Store, event stripe.Event) error {
	var session stripe.CheckoutSession
	if err := json.Unmarshal(event.Data.Raw, &session); err != nil {
		return fmt.Errorf("unmarshal checkout session: %w", err)
	}
	userID := session.Metadata["user_id"]
	tier := session.Metadata["tier"]
	if userID == "" || tier == "" {
		return fmt.Errorf("missing user_id or tier in session metadata")
	}
	return store.UpdateUserSubscription(userID, tier, "active")
}

func handleSubscriptionDeleted(store *db.Store, event stripe.Event) error {
	var sub stripe.Subscription
	if err := json.Unmarshal(event.Data.Raw, &sub); err != nil {
		return fmt.Errorf("unmarshal subscription: %w", err)
	}
	if sub.Customer == nil {
		return fmt.Errorf("subscription missing customer")
	}
	user, err := store.GetUserByStripeCustomerID(sub.Customer.ID)
	if err != nil || user == nil {
		return fmt.Errorf("user not found for customer %s", sub.Customer.ID)
	}
	return store.UpdateUserSubscription(user.ID, "free", "cancelled")
}

func handlePaymentFailed(store *db.Store, event stripe.Event) error {
	var invoice stripe.Invoice
	if err := json.Unmarshal(event.Data.Raw, &invoice); err != nil {
		return fmt.Errorf("unmarshal invoice: %w", err)
	}
	if invoice.Customer == nil {
		return fmt.Errorf("invoice missing customer")
	}
	user, err := store.GetUserByStripeCustomerID(invoice.Customer.ID)
	if err != nil || user == nil {
		return fmt.Errorf("user not found for customer %s", invoice.Customer.ID)
	}
	return store.UpdateUserSubscription(user.ID, user.SubscriptionTier, "past_due")
}
