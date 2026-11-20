package model

import (
	"math"
	"slices"
	"strings"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	"github.com/expr-lang/expr/ast"
	"github.com/expr-lang/expr/parser"
	"github.com/shopspring/decimal"
)

// RetailVideoPrices is a customer quote in the site's billing currency. Each
// group's amounts already include that customer's applicable group multiplier.
type RetailVideoPrices struct {
	Basis  string             `json:"basis,omitempty"`
	Groups []RetailVideoGroup `json:"groups"`
}

type RetailVideoGroup struct {
	Group      string           `json:"group"`
	PerRequest float64          `json:"per_request,omitempty"`
	Rows       []RetailVideoRow `json:"rows"`
}

type RetailVideoRow struct {
	Feature string             `json:"feature"`
	Prices  map[string]float64 `json:"prices"`
}

type klingVideoSpec struct {
	feature string
	// Absent resolutions are unsupported. Quantities are server-side only.
	units [3]float64
}

// Official resource usage per output second. Only the final group prices from
// the active sales expression leave this process.
var klingCreditSpecs = map[string][]klingVideoSpec{
	"kling-v3-omni": {
		{"No reference video · Native audio off", [3]float64{0.6, 0.8, 3}},
		{"No reference video · Native audio on", [3]float64{0.8, 1, 3}},
		{"Reference video · Native audio off", [3]float64{0.9, 1.2, 3}},
	},
	"kling-v3": {
		{"Native audio off", [3]float64{0.6, 0.8, 3}},
		{"Native audio on", [3]float64{0.9, 1.2, 3}},
		{"Motion control", [3]float64{0.9, 1.2}},
	},
	"kling-3.0": {{"Motion control", [3]float64{0.9, 1.2}}},
	"kling-v2-6": {
		{"Native audio off", [3]float64{0.3, 0.5}},
		{"Native audio on · No voice reference", [3]float64{0, 1}},
		{"Native audio on · With voice reference", [3]float64{0, 1.2}},
		{"Motion control", [3]float64{0.5, 0.8}},
	},
}

func klingRetailSpecs(modelName string) []klingVideoSpec {
	if modelName == "kling-3.0-omni" {
		modelName = "kling-v3-omni"
	}
	return klingCreditSpecs[modelName]
}

func HasPrivateVideoPricing(modelName string) bool {
	return modelName == "wan3.0-video" || modelName == "wan3.0-video-prime" || modelName == "kling-4.0" || modelName == "kling-4.0-flash" || len(klingRetailSpecs(modelName)) != 0
}

// The visual credit editor writes a linear completed-task price. Recognize
// that branch explicitly. A custom non-linear or request-dependent expression
// cannot truthfully be advertised as a single per-second price.
func klingRetailAmounts(expression string) (fixed, perUnit float64, ok bool) {
	version, body := billingexpr.ParseExprVersion(expression)
	if version != billingexpr.DefaultExprVersion {
		return 0, 0, false
	}
	tree, err := parser.Parse(body)
	if err != nil {
		return 0, 0, false
	}
	node := tree.Node
	if conditional, yes := node.(*ast.ConditionalNode); yes {
		switch conditional.Cond.String() {
		case `u("units_source") == "estimated"`:
			node = conditional.Exp2
		case `u("units_source") == "reported"`:
			node = conditional.Exp1
		default:
			return 0, 0, false
		}
	}
	tier, yes := node.(*ast.CallNode)
	if !yes || tier.Callee.String() != "tier" || len(tier.Arguments) != 2 {
		return 0, 0, false
	}
	amount := tier.Arguments[1]
	if add, yes := amount.(*ast.BinaryNode); yes && add.Operator == "+" {
		fixed, ok = klingPriceLiteral(add.Left)
		if !ok {
			return 0, 0, false
		}
		amount = add.Right
	}
	multiply, yes := amount.(*ast.BinaryNode)
	if !yes || multiply.Operator != "*" || multiply.Left.String() != `u("units")` {
		return 0, 0, false
	}
	perUnit, ok = klingPriceLiteral(multiply.Right)
	return fixed, perUnit, ok
}

func klingPriceLiteral(node ast.Node) (float64, bool) {
	switch number := node.(type) {
	case *ast.FloatNode:
		return number.Value, number.Value >= 0 && !math.IsInf(number.Value, 0)
	case *ast.IntegerNode:
		return float64(number.Value), number.Value >= 0
	}
	return 0, false
}

func videoRetailCondition(node ast.Node, resolution, input string) (bool, bool) {
	expr, ok := node.(*ast.BinaryNode)
	if !ok {
		return false, false
	}
	if expr.Operator == "&&" {
		a, okA := videoRetailCondition(expr.Left, resolution, input)
		b, okB := videoRetailCondition(expr.Right, resolution, input)
		return a && b, okA && okB
	}
	value, ok := expr.Right.(*ast.StringNode)
	if expr.Operator != "==" || !ok {
		return false, false
	}
	switch expr.Left.String() {
	case `u("resolution")`:
		return resolution == value.Value, true
	case `u("video_input")`:
		return input == value.Value, true
	}
	return false, false
}

// An arbitrary nonlinear expression cannot be displayed as a pair of unit prices.
func videoRetailCoefficients(node ast.Node) (base, hdr float64, ok bool) {
	if n, literal := klingPriceLiteral(node); literal {
		return 0, 0, n == 0
	}
	expr, ok := node.(*ast.BinaryNode)
	if !ok {
		return 0, 0, false
	}
	if expr.Operator == "+" {
		a, b, okA := videoRetailCoefficients(expr.Left)
		c, d, okB := videoRetailCoefficients(expr.Right)
		return a + c, b + d, okA && okB && !math.IsInf(a+c, 0) && !math.IsInf(b+d, 0)
	}
	n, ok := klingPriceLiteral(expr.Right)
	if expr.Operator != "*" || !ok {
		return 0, 0, false
	}
	switch expr.Left.String() {
	case `u("billed_seconds")`:
		return n, 0, true
	case `u("hdr_seconds")`:
		return 0, n, true
	}
	return 0, 0, false
}

func klingTimedRetailRows(modelName string, variant PricingPluginVariant) ([]RetailVideoRow, bool) {
	version, body := billingexpr.ParseExprVersion(variant.BillingExpr)
	if version != billingexpr.DefaultExprVersion {
		return nil, false
	}
	tree, err := parser.Parse(body)
	if err != nil {
		return nil, false
	}
	var rows []RetailVideoRow
	for _, input := range []string{"none", "video"} {
		feature, extra := "No input video · per output second", "HDR add-on without input video · per output second"
		if input == "video" {
			feature, extra = "With input video · per billable second", "HDR add-on with input video · per output second"
		}
		base := RetailVideoRow{Feature: feature, Prices: map[string]float64{}}
		hdr := RetailVideoRow{Feature: extra, Prices: map[string]float64{}}
		for _, res := range variant.BillingUsageSchema["resolution"].Enum {
			if res != "720p" && (modelName == "kling-4.0-flash" || res != "1080p" && res != "4k") {
				continue
			}
			if res == "4k" && !strings.Contains(body, `"4k"`) {
				continue
			}
			node := tree.Node
			for {
				cond, branched := node.(*ast.ConditionalNode)
				if !branched {
					break
				}
				match, valid := videoRetailCondition(cond.Cond, res, input)
				if !valid {
					return nil, false
				}
				if match {
					node = cond.Exp1
				} else {
					node = cond.Exp2
				}
			}
			tier, ok := node.(*ast.CallNode)
			if !ok || tier.Callee.String() != "tier" || len(tier.Arguments) != 2 {
				return nil, false
			}
			rate, supplement, ok := videoRetailCoefficients(tier.Arguments[1])
			if !ok {
				return nil, false
			}
			size := strings.ToUpper(res)
			base.Prices[size] = rate
			if _, supported := variant.BillingUsageSchema["hdr_seconds"]; supported && res != "720p" && strings.Contains(body, `u("hdr_seconds")`) {
				hdr.Prices[size] = supplement
			}
		}
		if len(base.Prices) == 0 {
			return nil, false
		}
		rows = append(rows, base)
		if len(hdr.Prices) > 0 {
			rows = append(rows, hdr)
		}
	}
	return rows, true
}

func wanRetailRows(variant PricingPluginVariant) ([]RetailVideoRow, bool) {
	version, body := billingexpr.ParseExprVersion(variant.BillingExpr)
	if version != billingexpr.DefaultExprVersion {
		return nil, false
	}
	tree, err := parser.Parse(body)
	if err != nil {
		return nil, false
	}
	row := RetailVideoRow{Feature: "Video generation", Prices: map[string]float64{}}
	for _, res := range variant.BillingUsageSchema["resolution"].Enum {
		if res != "480P" && res != "720P" && res != "1080P" {
			continue
		}
		node := tree.Node
		for {
			branch, ok := node.(*ast.ConditionalNode)
			if !ok {
				break
			}
			matches, valid := videoRetailCondition(branch.Cond, res, "")
			if !valid {
				return nil, false
			}
			if matches {
				node = branch.Exp1
			} else {
				node = branch.Exp2
			}
		}
		tier, ok := node.(*ast.CallNode)
		if !ok || tier.Callee.String() != "tier" || len(tier.Arguments) != 2 {
			return nil, false
		}
		rate, extra, ok := videoRetailCoefficients(tier.Arguments[1])
		if !ok || extra != 0 {
			return nil, false
		}
		row.Prices[res] = rate
	}
	return []RetailVideoRow{row}, len(row.Prices) > 0
}

// CustomerVideoPricing operates on a copy of the catalog entry, after the
// controller resolves the caller's actual group prices. Never publish a
// settlement expression, schema, provider variant or pre-discount price.
func CustomerVideoPricing(item Pricing, groupRatios map[string]float64) Pricing {
	specs := klingRetailSpecs(item.ModelName)
	if !HasPrivateVideoPricing(item.ModelName) {
		return item
	}
	wan := item.ModelName == "wan3.0-video" || item.ModelName == "wan3.0-video-prime"
	pluginKey := constant.KlingOfficialVideoPlugin
	if wan {
		pluginKey = constant.WanOfficialVideoPlugin
	}
	var selected PricingPluginVariant
	if !item.HasOrdinaryChannel && len(item.BillingPluginVariants) == 1 {
		variant := item.BillingPluginVariants[0]
		if variant.PluginKey == pluginKey {
			selected = variant
		}
	}
	item.BillingMode = "retail_video"
	item.BillingExpr, item.BillingUsageSchema, item.BillingUsageExamples, item.BillingPluginVariants = "", nil, nil, nil
	item.ModelRatio, item.ModelPrice, item.CompletionRatio = 0, 0, 0
	item.RetailVideoPrices = &RetailVideoPrices{Groups: []RetailVideoGroup{}}
	var rows []RetailVideoRow
	var fixed float64
	if wan {
		item.RetailVideoPrices.Basis = "billable"
		var ok bool
		rows, ok = wanRetailRows(selected)
		if !ok {
			return item
		}
	} else if len(specs) == 0 {
		item.RetailVideoPrices.Basis = "timed"
		var ok bool
		rows, ok = klingTimedRetailRows(item.ModelName, selected)
		if !ok {
			return item
		}
	} else {
		var perUnit float64
		var ok bool
		fixed, perUnit, ok = klingRetailAmounts(selected.BillingExpr)
		if !ok {
			return item
		}
		for _, spec := range specs {
			row := RetailVideoRow{Feature: spec.feature, Prices: map[string]float64{}}
			for i, size := range [...]string{"720P", "1080P", "4K"} {
				if spec.units[i] > 0 {
					rate := decimal.NewFromFloat(spec.units[i]).Mul(decimal.NewFromFloat(perUnit)).InexactFloat64()
					if math.IsInf(rate, 0) {
						return item
					}
					row.Prices[size] = rate
				}
			}
			rows = append(rows, row)
		}
	}
	var groups []string
	for group := range groupRatios {
		if slices.Contains(item.EnableGroup, "all") || slices.Contains(item.EnableGroup, group) {
			groups = append(groups, group)
		}
	}
	slices.Sort(groups)
	for _, group := range groups {
		ratio := groupRatios[group]
		if ratio < 0 || math.IsNaN(ratio) || math.IsInf(ratio, 0) {
			continue
		}
		groupRate := decimal.NewFromFloat(ratio)
		quote := RetailVideoGroup{Group: group, PerRequest: decimal.NewFromFloat(fixed).Mul(groupRate).InexactFloat64()}
		if math.IsInf(quote.PerRequest, 0) {
			continue
		}
		for _, base := range rows {
			row := RetailVideoRow{Feature: base.Feature, Prices: map[string]float64{}}
			for size, rate := range base.Prices {
				quotePrice := decimal.NewFromFloat(rate).Mul(groupRate).InexactFloat64()
				if math.IsInf(quotePrice, 0) {
					return item
				}
				row.Prices[size] = quotePrice
			}
			quote.Rows = append(quote.Rows, row)
		}
		item.RetailVideoPrices.Groups = append(item.RetailVideoPrices.Groups, quote)
	}
	return item
}
