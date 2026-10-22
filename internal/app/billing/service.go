package billing

import (
	"context"
	"encoding/json"
	"math"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/pramaan-oss/cold-email/internal/app/discount"
	"github.com/pramaan-oss/cold-email/internal/app/worker"
	"github.com/pramaan-oss/cold-email/internal/config"
	"github.com/pramaan-oss/cold-email/internal/errx"
	"github.com/pramaan-oss/cold-email/internal/models"
	"github.com/pramaan-oss/cold-email/internal/observability/analytics"
	"github.com/pramaan-oss/cold-email/internal/pramaan"
	"github.com/pramaan-oss/cold-email/internal/repository"
)

type Customer struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
}
type CheckoutSession struct {
	ID  string `json:"ref"`
	URL string `json:"checkoutUrl"`
}
type Subscription struct {
	ID                string `json:"id"`
	Status            string `json:"status"`
	CancelAtPeriodEnd bool   `json:"cancelAtPeriodEnd"`
	CurrentPeriodEnd  int64  `json:"currentPeriodEnd"`
}
type Event struct {
	ID, Type string
	Data     json.RawMessage
}
type ProrationPreview struct {
	CurrentPlan     *models.Plan `json:"current_plan"`
	NewPlan         *models.Plan `json:"new_plan"`
	ProrationAmount int64        `json:"proration_amount"`
	AmountDue       int64        `json:"amount_due"`
	NextBillingDate time.Time    `json:"next_billing_date"`
	Currency        string       `json:"currency"`
}

type BillingService interface {
	// Customer management
	CreateCustomer(ctx context.Context, userID uuid.UUID, email, name string) (string, *errx.Error)
	GetCustomer(ctx context.Context, customerID string) (*Customer, *errx.Error)

	// Checkout. discountCode is optional ("" for none); when set, a one-off
	// Billing coupon (money discounts) or trial extension is applied.
	CreateCheckoutSession(ctx context.Context, userID uuid.UUID, orgID uuid.UUID, priceID, successURL, cancelURL, discountCode string) (*CheckoutSession, *errx.Error)
	CreatePortalSession(ctx context.Context, customerID, returnURL string) (string, *errx.Error)

	// Subscriptions
	GetSubscription(ctx context.Context, subscriptionID string) (*Subscription, *errx.Error)
	CancelSubscription(ctx context.Context, subscriptionID string, cancelAtPeriodEnd bool) *errx.Error

	// Plan changes with proration. discountCode is optional ("" for none).
	ChangePlan(ctx context.Context, orgID uuid.UUID, newPlanID uuid.UUID, prorationBehavior, discountCode, interval string) (*Subscription, *errx.Error)
	PreviewPlanChange(ctx context.Context, orgID uuid.UUID, newPlanID uuid.UUID) (*ProrationPreview, *errx.Error)

	// Webhooks
	VerifyWebhook(payload []byte, signature string) (*Event, *errx.Error)
	ProcessWebhookEvent(ctx context.Context, event *Event) *errx.Error

	// ApplyCustomerCredit adds a signed cents delta to a customer's Billing
	// balance (negative = credit the customer). Satisfies referral.BillingBalancer.
	ApplyCustomerCredit(ctx context.Context, customerID string, amountCents int64, currency, idempotencyKey string) (string, *errx.Error)

	// CreateCreditCheckoutSession creates a one-time (mode=payment) Billing
	// Checkout session to buy a top-up credit pack, reusing the org's existing
	// Billing customer. Fulfillment happens only in the webhook. packKey is a
	// credits.CreditPack.Key; credits is the pack's credit count (recorded in
	// session metadata so the webhook grants the right amount). The pack's
	// Billing price is resolved from config; an unconfigured pack returns 503.
	CreateCreditCheckoutSession(ctx context.Context, userID, orgID uuid.UUID, packKey string, credits int, successURL, cancelURL string) (*CheckoutSession, *errx.Error)

	// AutoTopUpCredits charges the org's saved payment method OFF-SESSION for
	// one credit pack and fulfills it immediately on success (idempotent on
	// the PaymentIntent id). Used by the credit-watch auto top-up; returns
	// true when credits were granted. Fails cleanly (false, err) when the org
	// has no saved payment method or the charge is declined.
	AutoTopUpCredits(ctx context.Context, orgID uuid.UUID, packKey string, creditAmount int) (bool, error)

	// WireReferral attaches the referral program (post-construction; nil = the
	// referral hooks in the webhook flow are skipped).
	WireReferral(r ReferralRewarder)

	// WireAnalytics attaches the product-analytics sink (post-construction;
	// nil = money events are not counted, which is the self-host default).
	WireAnalytics(a ProductAnalytics)

	// WireCredits attaches the AI-credit granter and an audit logger
	// (post-construction; nil = the credit grant/reset hooks are skipped).
	WireCredits(g CreditGranter, a AuditLogger)
}

// CreditGranter is the slice of the credit service the Billing webhook flow
// drives: reset the monthly allowance each billing cycle and fulfill top-up
// purchases. *credits.creditService satisfies it (plain error returns).
type CreditGranter interface {
	ResetMonthlyAllowance(ctx context.Context, orgID uuid.UUID, allowance int, idempotencyKey string) error
	GrantPurchased(ctx context.Context, orgID uuid.UUID, amount int, reason, idempotencyKey string) (int, error)
}

// AuditLogger is the slice of the audit service the webhook flow uses to fire
// AUDIT_CREATED so teammates' billing/credits views refresh live after a grant
// or purchase. *audit.auditService satisfies it.
type AuditLogger interface {
	LogAction(ctx context.Context, orgID, actorID uuid.UUID, action models.AuditAction, entityType models.AuditEntityType, entityID *uuid.UUID, ip, userAgent string, changes, metadata map[string]string)
}

// ReferralRewarder is the slice of the referral service the Billing webhook flow
// drives. *referral.Service satisfies it; injected via WireReferral so the
// billing package needs no import of referral (no cycle).
type ReferralRewarder interface {
	QualifyOnConversion(ctx context.Context, inviteeOrgID uuid.UUID)
	RewardOnFirstInvoice(ctx context.Context, inviteeOrgID, planID uuid.UUID, eventID string) *errx.Error
	ClawbackForInvitee(ctx context.Context, inviteeOrgID uuid.UUID, eventID, reason string)
	SyncBillingBalance(ctx context.Context, orgID uuid.UUID)
	InviteeDiscountCode(ctx context.Context, inviteeOrgID uuid.UUID) string
}

// OperatorNotifier is the instance-wide operator alert surface, injected
// post-construction so this package needs no import of it. Nil disables it.
type OperatorNotifier interface {
	NotifyOperator(key, title, summary string, fields map[string]string)
}

// ProductAnalytics counts a started subscription. Satisfied by
// *analytics.Client; nil-safe, so an instance with no external telemetry configuration counts
// nothing. Properties never name an organization or a person.
type ProductAnalytics interface {
	Capture(name string, req analytics.Request, properties map[string]any)
}

type billingService struct {
	client   pramaan.Caller
	subRepo  repository.SubscriptionRepository
	planRepo repository.PlanRepository
}

func NewService(_ *config.BillingConfig, sub repository.SubscriptionRepository, plans repository.PlanRepository, _ worker.WorkerAssignmentService, _ discount.DiscountService, _ repository.CreditAutoTopUpAttemptRepository) BillingService {
	c, _ := pramaan.FromEnv()
	return &billingService{client: c, subRepo: sub, planRepo: plans}
}
func (s *billingService) call(ctx context.Context, id string, in, out any, key string) *errx.Error {
	if s.client == nil {
		return errx.New(errx.ServiceUnavailable, "Connect Pramaan to use billing.")
	}
	if e := s.client.Call(ctx, id, in, out, pramaan.Idempotency(key)); e != nil {
		return errx.New(errx.ServiceUnavailable, e.Error())
	}
	return nil
}
func (s *billingService) CreateCustomer(ctx context.Context, user uuid.UUID, email, name string) (string, *errx.Error) {
	var out struct {
		CustomerID string `json:"customerId"`
		Ref        string `json:"ref"`
	}
	e := s.call(ctx, "billing.customer.create", map[string]any{"email": email, "name": name}, &out, "customer-"+user.String())
	id := out.CustomerID
	if id == "" {
		id = out.Ref
	}
	return id, e
}
func (s *billingService) GetCustomer(ctx context.Context, id string) (*Customer, *errx.Error) {
	out := &Customer{}
	e := s.call(ctx, "billing.customer.get", map[string]any{"customerId": id}, out, uuid.NewString())
	return out, e
}
func (s *billingService) CreatePortalSession(ctx context.Context, id, returnURL string) (string, *errx.Error) {
	var out struct {
		URL string `json:"url"`
	}
	e := s.call(ctx, "billing.portal.create", map[string]any{"customerId": id, "returnUrl": returnURL}, &out, uuid.NewString())
	return out.URL, e
}
func (s *billingService) GetSubscription(ctx context.Context, id string) (*Subscription, *errx.Error) {
	out := &Subscription{}
	e := s.call(ctx, "billing.subscription.get", map[string]any{"subscriptionId": id}, out, uuid.NewString())
	return out, e
}
func (s *billingService) CancelSubscription(ctx context.Context, id string, atEnd bool) *errx.Error {
	return s.call(ctx, "billing.subscription.cancel", map[string]any{"subscriptionId": id, "atPeriodEnd": atEnd}, nil, "cancel-"+id)
}
func (s *billingService) CreateCheckoutSession(ctx context.Context, user, org uuid.UUID, priceID, successURL, cancelURL, discountCode string) (*CheckoutSession, *errx.Error) {
	if discountCode != "" {
		return nil, errx.New(errx.ServiceUnavailable, "Checkout discount coverage needs verification.")
	}
	// Resolve a product's catalog price through Pramaan; never use the original hosted price IDs.
	var price map[string]any
	if e := s.call(ctx, "billing.price.get", map[string]any{"priceId": priceID}, &price, uuid.NewString()); e != nil {
		return nil, e
	}
	amount, ok := price["unitPrice"].(float64)
	currency, _ := price["currency"].(string)
	name, _ := price["name"].(string)
	if name == "" {
		name = "Subscription"
	}
	interval, _ := price["renewalIntervalUnit"].(string)
	count, _ := price["renewalIntervalCount"].(float64)
	if !ok || amount < 1 || amount > 99999999 || math.Trunc(amount) != amount || len(currency) != 3 || strings.Trim(currency, "abcdefghijklmnopqrstuvwxyz") != "" || (interval != "day" && interval != "week" && interval != "month" && interval != "year") || count != 1 || price["scheme"] != "standard" || price["category"] != "subscription" || price["setupFeeEnabled"] == true || price["trialIntervalUnit"] != nil {
		return nil, errx.New(errx.ServiceUnavailable, "This price needs coverage verification.")
	}
	var out CheckoutSession
	e := s.call(ctx, "payments.checkout.create", map[string]any{"lineItems": []map[string]any{{"name": name, "amount": amount, "currency": currency, "interval": interval, "quantity": 1}}, "mode": "subscription", "successUrl": successURL, "cancelUrl": cancelURL}, &out, uuid.NewString())
	if e != nil {
		return nil, e
	}
	target, parseErr := url.Parse(out.URL)
	if out.ID == "" || parseErr != nil || target.Scheme != "https" || target.Host == "" || target.User != nil {
		return nil, errx.New(errx.ServiceUnavailable, "Checkout could not be confirmed. Check its status before starting another checkout.")
	}
	return &out, nil
}
func (s *billingService) ChangePlan(ctx context.Context, org, plan uuid.UUID, _, _, _ string) (*Subscription, *errx.Error) {
	return nil, errx.New(errx.ServiceUnavailable, "Choose a catalog plan through your billing connection.")
}
func (s *billingService) PreviewPlanChange(context.Context, uuid.UUID, uuid.UUID) (*ProrationPreview, *errx.Error) {
	return nil, errx.New(errx.ServiceUnavailable, "A prorated preview is not available yet.")
}
func (s *billingService) VerifyWebhook([]byte, string) (*Event, *errx.Error) {
	return nil, errx.New(errx.ServiceUnavailable, "Direct billing webhooks are unavailable. Use broker events.")
}
func (s *billingService) ProcessWebhookEvent(context.Context, *Event) *errx.Error {
	return errx.New(errx.ServiceUnavailable, "Billing event coverage needs verification.")
}
func (s *billingService) ApplyCustomerCredit(context.Context, string, int64, string, string) (string, *errx.Error) {
	return "", errx.New(errx.ServiceUnavailable, "Balance adjustments are not available yet.")
}
func (s *billingService) CreateCreditCheckoutSession(context.Context, uuid.UUID, uuid.UUID, string, int, string, string) (*CheckoutSession, *errx.Error) {
	return nil, errx.New(errx.ServiceUnavailable, "Credit fulfillment needs event coverage verification.")
}
func (s *billingService) AutoTopUpCredits(context.Context, uuid.UUID, string, int) (bool, error) {
	return false, pramaan.ErrUnavailable
}
func (s *billingService) WireReferral(ReferralRewarder)          {}
func (s *billingService) WireAnalytics(ProductAnalytics)         {}
func (s *billingService) WireCredits(CreditGranter, AuditLogger) {}
func (s *billingService) WireOperatorNotifier(OperatorNotifier)  {}
