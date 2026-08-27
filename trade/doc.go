// Package trade provides access to Webull's Trading API: accounts, balances,
// positions, cash activities, and reference data for tradable instruments.
//
// Obtain a Client from the root package rather than constructing one here:
//
//	client, err := webull.NewClient(webull.Config{...})
//	accounts, err := client.Trade.Accounts(ctx)
//
// All methods take a context.Context and are safe for concurrent use.
//
// # Numbers
//
// Webull transmits every price, quantity and monetary amount as a decimal
// string. This package preserves that exactness with decimal.Decimal for
// fields that are always present and decimal.NullDecimal where a field may be
// absent, so that "not reported" is distinguishable from zero. No field is
// ever a float.
//
// # Pagination
//
// Webull uses two mechanisms, and this package exposes each as it is rather
// than hiding the difference. Instrument listings return a PaginationKey that
// is passed back to fetch the next page and is empty on the last one. Cash
// activities use the ID of the last item seen. No method fetches more than one
// page on its own.
package trade
