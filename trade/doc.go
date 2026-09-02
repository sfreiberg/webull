// Package trade provides access to Webull's Trading API: accounts, balances,
// positions, cash activities, reference data for tradable instruments, and
// orders across every asset class Webull offers.
//
// Obtain a Client from the root package rather than constructing one here:
//
//	client, err := webull.NewClient(webull.Config{...})
//	accounts, err := client.Trade.Accounts(ctx)
//
// All methods take a context.Context and are safe for concurrent use.
//
// # Orders
//
// An Order describes an order to preview or place; PreviewOrder and
// PlaceOrder take a pointer because they fill in defaults and generated
// values that the caller needs afterwards. Only the fields a plain order
// needs must be set; the rest of Webull's required boilerplate is supplied.
//
// Orders are validated locally before any request is sent, and a malformed
// order fails with ErrInvalidOrder without touching the network. The rules
// are Webull's: which sides, order types, times in force and quantity
// precision each asset class permits, which fields each order type needs,
// and how groups and multi-leg strategies may be shaped. Where Webull's
// written documentation and its API disagree, the API wins; each rule here
// was checked against the sandbox where that was possible.
//
// # Order safety
//
// Every order carries a ClientOrderID, which is the key for lookup, replace
// and cancel. If the caller leaves it empty one is generated and written back
// to the Order before the request is sent, so the caller holds it even when
// no response arrives. That matters because PlaceOrder never retries: in this
// API a replayed placement is a duplicated order. After a transport error the
// outcome is unknown, and the right move is to look the order up by its
// ClientOrderID rather than to place it again.
//
// A ClientOrderID is consumed once Webull accepts the order. Placing the same
// Order value a second time returns ErrDuplicateOrder. A rejected placement
// does not consume the ID, so a corrected Order may be resent as is.
//
// # Asset classes
//
// Set Order.InstrumentType to trade futures, crypto or event contracts;
// equities are the default and options are inferred from Legs. Futures,
// crypto and event contracts each have their own account, with the matching
// AccountClass, and an order must be placed against the right one.
//
// Some rules differ from Webull's prose and follow what the API does:
// options accept market orders and GTC sells, which the FAQ says they do
// not; event contracts accept GTC, which the guide says they do not; and a
// trailing stop on an option is rejected, which both agree on.
//
// # Groups and strategies
//
// Bracket, OTO, OCO and OTOCO build a Combo of related orders that
// PlaceCombo submits as one call. Multi-leg option strategies are a single
// Order with an OptionStrategy and several Legs; LegFromSymbol builds a leg
// from the OCC symbols that OptionContracts returns.
//
// # Sandbox
//
// Webull's sandbox enforces regular trading hours for DAY orders, so tests
// and tooling that must run at any hour should use GTC. It also refuses OTO,
// OCO and OTOCO groups, batch placement, and every crypto preview, all of
// which the documentation describes as supported. Those are recorded in the
// repository's compatibility matrix and the affected features are
// implemented to the documentation but unverified.
//
// # Numbers
//
// Webull transmits every price, quantity and monetary amount as a decimal
// string. This package preserves that exactness with the SDK's Decimal type
// for fields that are always present and NullDecimal where a field may be
// absent, so that "not reported" is distinguishable from zero. Both embed
// shopspring's decimal types, so all their methods are available. No field is
// ever a float. Price builds a set NullDecimal from a literal.
//
// # Pagination
//
// Webull uses two mechanisms, and this package exposes each as it is rather
// than hiding the difference. Instrument listings return a PaginationKey that
// is passed back to fetch the next page and is empty on the last one. Cash
// activities and order listings page by the ID of the last item seen. No
// method fetches more than one page on its own.
package trade
