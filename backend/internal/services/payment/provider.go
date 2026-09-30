package payment

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type Provider interface {
	Name() string
	Charge(ctx context.Context, req ChargeRequest) (ChargeResult, error)
}

type ChargeRequest struct {
	PaymentNo string
	OrderID   uint
	UserID    uint
	Amount    int
}

type ChargeResult struct {
	Success     bool
	ProviderRef string
	FailReason  string
}

// MockProvider always succeeds (demo).
type MockProvider struct{}

func (MockProvider) Name() string { return "mock" }

func (MockProvider) Charge(ctx context.Context, req ChargeRequest) (ChargeResult, error) {
	select {
	case <-ctx.Done():
		return ChargeResult{}, ctx.Err()
	case <-time.After(20 * time.Millisecond):
	}
	return ChargeResult{
		Success:     true,
		ProviderRef: fmt.Sprintf("MOCK-%s", uuid.NewString()[:8]),
	}, nil
}

// FailProvider for tests.
type FailProvider struct{ Reason string }

func (p FailProvider) Name() string { return "fail" }

func (p FailProvider) Charge(ctx context.Context, req ChargeRequest) (ChargeResult, error) {
	reason := p.Reason
	if reason == "" {
		reason = "支付失败"
	}
	return ChargeResult{Success: false, FailReason: reason}, nil
}

func New(name string) (Provider, error) {
	switch name {
	case "", "mock":
		return MockProvider{}, nil
	case "fail":
		return FailProvider{}, nil
	default:
		return nil, errors.New("unknown payment provider: " + name)
	}
}
