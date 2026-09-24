package operation_setting

// GetGPTImage1PriceOnceCall retains the local per-call pricing used by legacy
// image generation tool billing.
func GetGPTImage1PriceOnceCall(quality, size string) float64 {
	prices := map[string]map[string]float64{
		"low": {"1024x1024": 0.011, "1024x1536": 0.016, "1536x1024": 0.016},
		"medium": {"1024x1024": 0.042, "1024x1536": 0.063, "1536x1024": 0.063},
		"high": {"1024x1024": 0.167, "1024x1536": 0.25, "1536x1024": 0.25},
	}
	if qualityPrices, ok := prices[quality]; ok {
		if price, ok := qualityPrices[size]; ok {
			return price
		}
	}
	return 0.167
}
