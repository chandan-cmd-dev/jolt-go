package apdctx

import "github.com/cockroachdb/apd/v3"

// Shared context for decimal math: 34 digits (decimal128-like), bankers rounding.
var Ctx = apd.Context{
    Precision:   34,
    MaxExponent: apd.MaxExponent,
    MinExponent: apd.MinExponent,
    Rounding:    apd.RoundHalfEven,
}
