# Wire format: prices, quantities and timestamps

This document exists to settle one decision — how the SDK represents monetary
and quantity values — because §23 of the design proposal flags it and because it
is the most expensive thing to change once order and quote models are written.

## What Webull actually sends

**Decimal values are transmitted as strings, everywhere.**

Evidence, from three independent directions:

**1. The protobuf definitions.** `message.proto`, which defines every real-time
market-data payload, types every numeric field as `string`:

```proto
message Snapshot {
  Basic basic = 1;
  string trade_time = 2;
  string price = 3;
  string open = 4;
  string high = 5;
  string low = 6;
  string pre_close = 7;
  string volume = 8;
  string change = 9;
  string change_ratio = 10;
  ...
}
```

Protobuf has `double`, `float`, `sint64` and well-known decimal conventions
available. Webull chose `string` for prices, volumes and ratios deliberately.

**2. The Java SDK's domain models.** Counting field declarations across every
model class:

| Declared type | Count |
|---|---|
| `String` | 1235 |
| `int` | 42 |
| `Integer` | 40 |
| `Boolean` | 22 |
| `Long` | 14 |
| `long` | 6 |
| `Double` | 4 |
| `BigDecimal` | 2 |

Every price, quantity, market value, filled quantity, average fill price and cost
basis is a `String`. The six non-integral outliers are instructive: `Double size`
and `Double minTick` on a futures instrument, `Double from`/`to` on a fund split
ratio, and two `BigDecimal` fields that are *query parameters* for filtering
option contracts by strike, not response values.

**3. Webull's own Go CLI** carries no floating-point price handling at all.

## What this means

The question §23 poses — "should prices be strings, a decimal type, integers
plus scale, or another exact representation" — is partly answered for us. Webull
is already handing us exact decimal strings. Any float64 in our models would be
a precision loss *we introduce*, converting an exact value into an inexact one
and back.

The remaining question is what we hand to the caller.

## Options

**A. Preserve strings in models, offer conversion helpers.**
Field is `string`; a method or free function converts to a decimal type on
demand. Zero dependencies, zero precision loss, round-trips byte-exactly.
Downside: callers do arithmetic themselves, and `Price string` invites someone
to `strconv.ParseFloat` it anyway.

**B. A decimal type in the public models.**
Field is `decimal.Decimal` (shopspring) or similar. Ergonomic and safe for
arithmetic. Downsides: a dependency in every public signature, which §47 asks us
to justify carefully; it becomes permanently part of our API compatibility
promise; and round-tripping is not guaranteed byte-exact — `"1.50"` may
re-serialize as `"1.5"`, which matters when the value is fed back into a signed
order body whose digest must match.

**C. A small internal decimal type of our own.**
Wraps the original string, exposes exact accessors, preserves the source
representation for round-tripping. No external dependency, full control.
Downside: we own a numeric type, which is a real maintenance burden and easy to
get subtly wrong.

## Recommendation

**Option A for v0.x, with the door open to C.**

Rationale: it is the only option that cannot lose information, it adds no
dependency to the public API, and it round-trips exactly — which matters more
here than usual, because request bodies are covered by a SHA-256 digest in the
signature. If we reformat a decimal on the way out, the signature covers bytes
that differ from what the caller supplied. Strings sidestep that entirely.

To address the ergonomic gap, define a named type rather than bare `string`:

```go
// Decimal is an exact decimal value as transmitted by Webull. It preserves the
// wire representation so that values round-trip byte-exactly, which matters for
// request bodies covered by the request signature.
type Decimal string

func (d Decimal) Float64() (float64, error)   // documented as lossy
func (d Decimal) BigRat() (*big.Rat, bool)
func (d Decimal) String() string
```

That gives us a place to hang conversion helpers and documentation, makes
`Price Decimal` self-describing at the call site, and costs nothing. If we later
decide callers need real arithmetic, `Decimal` can gain methods without breaking
the field type.

**This is a decision gate.** It should be confirmed before Phase 3 writes the
first model, because reversing it later touches every order, position and quote
type in the SDK.

## Timestamps

Less settled. Observed forms:

- `string timestamp` and `string trade_time` in the protobuf payloads.
- `int64 timestamp` in the gRPC `SubscribeRequest` and `SubscribeResponse`.
- `Long` fields in some Java models.

So timestamps are epoch integers in some places and formatted strings in others,
and the string formats have not been verified across endpoints. §23 says to use
`time.Time` where semantics are clear and stable, and to preserve the raw value
otherwise.

**Recommendation:** defer. Catalogue the actual format per endpoint during
Phase 3 as real responses become available, then decide. Where a field is an
unambiguous epoch integer, `time.Time` with a documented precision is safe. Where
it is a string of unverified format, keep the raw value and add a helper. Do not
guess a layout string from a single example — a wrong timezone assumption on an
order timestamp is a genuinely harmful bug.
