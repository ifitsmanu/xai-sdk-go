package manage

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/ifitsmanu/xai-sdk-go/internal/rest"
)

// Billing is the team billing API.
type Billing struct {
	c *Client
}

// Info is the billing profile. Field names match the management JSON.
type Info struct {
	Name      string   `json:"name,omitempty"`
	Email     string   `json:"email,omitempty"`
	Address   *Address `json:"address,omitempty"`
	TaxIDType string   `json:"taxIdType,omitempty"`
	TaxNumber string   `json:"taxNumber,omitempty"`
}

// Address is a billing address.
type Address struct {
	Line1      string `json:"line1,omitempty"`
	Line2      string `json:"line2,omitempty"`
	City       string `json:"city,omitempty"`
	Country    string `json:"country,omitempty"`
	PostalCode string `json:"postalCode,omitempty"`
	State      string `json:"state,omitempty"`
}

// GetInfo calls GET /v1/billing/teams/{teamID}/billing-info.
func (s *Billing) GetInfo(ctx context.Context, teamID string) (*Info, error) {
	var wrap struct {
		BillingInfo Info `json:"billingInfo"`
	}
	if err := s.c.rest.Call(ctx, http.MethodGet, rest.Path("/v1/billing/teams", teamID, "billing-info"), nil, &wrap); err != nil {
		return nil, err
	}
	return &wrap.BillingInfo, nil
}

// SetInfo calls POST /v1/billing/teams/{teamID}/billing-info.
func (s *Billing) SetInfo(ctx context.Context, teamID string, info Info) (*Info, error) {
	var wrap struct {
		BillingInfo Info `json:"billingInfo"`
	}
	body := struct {
		BillingInfo Info `json:"billingInfo"`
	}{BillingInfo: info}
	if err := s.c.rest.Call(ctx, http.MethodPost, rest.Path("/v1/billing/teams", teamID, "billing-info"), body, &wrap); err != nil {
		return nil, err
	}
	return &wrap.BillingInfo, nil
}

// Invoices calls GET /v1/billing/teams/{teamID}/invoices.
func (s *Billing) Invoices(ctx context.Context, teamID string) (json.RawMessage, error) {
	return s.raw(ctx, http.MethodGet, rest.Path("/v1/billing/teams", teamID, "invoices"), nil)
}

// PaymentMethod calls GET /v1/billing/teams/{teamID}/payment-method.
func (s *Billing) PaymentMethod(ctx context.Context, teamID string) (json.RawMessage, error) {
	return s.raw(ctx, http.MethodGet, rest.Path("/v1/billing/teams", teamID, "payment-method"), nil)
}

// SetDefaultPaymentMethod calls POST /v1/billing/teams/{teamID}/payment-method/default.
func (s *Billing) SetDefaultPaymentMethod(ctx context.Context, teamID string, body any) (json.RawMessage, error) {
	return s.raw(ctx, http.MethodPost, rest.Path("/v1/billing/teams", teamID, "payment-method", "default"), body)
}

// InvoicePreview calls GET /v1/billing/teams/{teamID}/postpaid/invoice/preview.
func (s *Billing) InvoicePreview(ctx context.Context, teamID string) (json.RawMessage, error) {
	return s.raw(ctx, http.MethodGet, rest.Path("/v1/billing/teams", teamID, "postpaid", "invoice", "preview"), nil)
}

// SpendingLimits calls GET /v1/billing/teams/{teamID}/postpaid/spending-limits.
func (s *Billing) SpendingLimits(ctx context.Context, teamID string) (json.RawMessage, error) {
	return s.raw(ctx, http.MethodGet, rest.Path("/v1/billing/teams", teamID, "postpaid", "spending-limits"), nil)
}

// SetSpendingLimits calls POST /v1/billing/teams/{teamID}/postpaid/spending-limits.
func (s *Billing) SetSpendingLimits(ctx context.Context, teamID string, body any) (json.RawMessage, error) {
	return s.raw(ctx, http.MethodPost, rest.Path("/v1/billing/teams", teamID, "postpaid", "spending-limits"), body)
}

// Balance calls GET /v1/billing/teams/{teamID}/prepaid/balance.
func (s *Billing) Balance(ctx context.Context, teamID string) (json.RawMessage, error) {
	return s.raw(ctx, http.MethodGet, rest.Path("/v1/billing/teams", teamID, "prepaid", "balance"), nil)
}

// TopUp calls POST /v1/billing/teams/{teamID}/prepaid/top-up.
func (s *Billing) TopUp(ctx context.Context, teamID string, body any) (json.RawMessage, error) {
	return s.raw(ctx, http.MethodPost, rest.Path("/v1/billing/teams", teamID, "prepaid", "top-up"), body)
}

// Usage calls POST /v1/billing/teams/{teamID}/usage.
func (s *Billing) Usage(ctx context.Context, teamID string, body any) (json.RawMessage, error) {
	return s.raw(ctx, http.MethodPost, rest.Path("/v1/billing/teams", teamID, "usage"), body)
}

func (s *Billing) raw(ctx context.Context, method, path string, in any) (json.RawMessage, error) {
	var out json.RawMessage
	if err := s.c.rest.Call(ctx, method, path, in, &out); err != nil {
		return nil, err
	}
	return out, nil
}
