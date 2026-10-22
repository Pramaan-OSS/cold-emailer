package config

import "context"

type BillingConfig struct {

	// CreditPackPriceIDs maps a credit-pack key (credits.CreditPack.Key, e.g.
	// "pack_500") to its Billing one-time price id. Optional: an unset pack is
	// simply not purchasable. Loaded from BILLING_CREDIT_PACK_<CREDITS>_PRICE_ID.
	CreditPackPriceIDs map[string]string
}

func (c *Config) LoadBillingConfig(ctx context.Context) (*BillingConfig, error) {
	return &BillingConfig{CreditPackPriceIDs: map[string]string{
		"pack_500":   c.GetStringOptional(ctx, "BILLING_CREDIT_PACK_500_PRICE_ID", "", ""),
		"pack_2000":  c.GetStringOptional(ctx, "BILLING_CREDIT_PACK_2000_PRICE_ID", "", ""),
		"pack_10000": c.GetStringOptional(ctx, "BILLING_CREDIT_PACK_10000_PRICE_ID", "", ""),
	}}, nil
}
