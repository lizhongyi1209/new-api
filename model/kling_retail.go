package model

import (
	"math"
	"slices"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	"github.com/expr-lang/expr/ast"
	"github.com/expr-lang/expr/parser"
	"github.com/shopspring/decimal"
)

// RetailVideoPrices is a customer quote in the site's billing currency. Each
// group's amounts already include that customer's applicable group multiplier.
type RetailVideoPrices struct {
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

func HasPrivateKlingPricing(modelName string) bool {
	return len(klingRetailSpecs(modelName)) != 0
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

// CustomerKlingPricing operates on a copy of the catalog entry, after the
// controller resolves the caller's actual group prices. Never publish a
// settlement expression, schema, provider variant or pre-discount price.
func CustomerKlingPricing(item Pricing, groupRatios map[string]float64) Pricing {
	specs := klingRetailSpecs(item.ModelName)
	if len(specs) == 0 {
		return item
	}
	var expr string
	if !item.HasOrdinaryChannel && len(item.BillingPluginVariants) == 1 {
		variant := item.BillingPluginVariants[0]
		if variant.PluginKey == constant.KlingOfficialVideoPlugin {
			expr = variant.BillingExpr
		}
	}
	item.BillingMode = "retail_video"
	item.BillingExpr, item.BillingUsageSchema, item.BillingUsageExamples, item.BillingPluginVariants = "", nil, nil, nil
	item.ModelRatio, item.ModelPrice, item.CompletionRatio = 0, 0, 0
	item.RetailVideoPrices = &RetailVideoPrices{Groups: []RetailVideoGroup{}}
	fixed, perUnit, ok := klingRetailAmounts(expr)
	if !ok {
		return item
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
		unitPrice := decimal.NewFromFloat(perUnit).Mul(groupRate)
		quote := RetailVideoGroup{Group: group, PerRequest: decimal.NewFromFloat(fixed).Mul(groupRate).InexactFloat64()}
		if math.IsInf(quote.PerRequest, 0) {
			continue
		}
		for _, spec := range specs {
			row := RetailVideoRow{Feature: spec.feature, Prices: make(map[string]float64)}
			for i, resolution := range [...]string{"720P", "1080P", "4K"} {
				if spec.units[i] > 0 {
					price := decimal.NewFromFloat(spec.units[i]).Mul(unitPrice).InexactFloat64()
					if math.IsInf(price, 0) {
						return item
					}
					row.Prices[resolution] = price
				}
			}
			quote.Rows = append(quote.Rows, row)
		}
		item.RetailVideoPrices.Groups = append(item.RetailVideoPrices.Groups, quote)
	}
	return item
}
