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

**2. The Java SDK's domain models.** Every monetary and quantity field is
declared `String`. The distribution across all model classes:

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
already hands us exact decimal values. Any `float64` in our models would be a
precision loss *we introduce*, turning an exact value inexact and back again.

So `float64` is ruled out. The remaining question is which exact representation
we hand to the caller.

## Options

**A. Preserve wire strings, convert on demand.**
Field is a named `string` type with conversion helpers. No dependency, no
precision loss, round-trips byte-exactly. Downside: every caller writes the same
conversion, and a `string` price invites someone to reach for
`strconv.ParseFloat` and reintroduce the problem we were avoiding.

**B. A third-party decimal type in the public models.**
Ergonomic and exact. Costs a dependency that appears in public signatures and so
becomes part of the API compatibility promise under §42.

**C. Our own decimal type.**
Full control, no dependency, but we would own a numeric type — a real
maintenance burden and easy to get subtly wrong.

## Recommendation: `github.com/shopspring/decimal`

Option B, using the ecosystem standard.

It clears §47's bar that every dependency have a clear reason:

- **Exact.** A 29-digit value round-trips untouched. `float64` cannot make that
  claim, and float64 is what callers will otherwise reach for.
- **Marshals as a JSON string**, which is exactly Webull's wire format, so it
  drops into request bodies without special casing.
- **Stable.** v1 for years, currently v1.4.0, no v2 has ever been published. The
  API-compatibility risk that argues against putting a dependency in public
  signatures is empirically small here.
- **Precedent.** Alpaca's Go SDK — the closest comparable broker client — uses it
  directly across 30 public entity fields.
- **Maintained.** ~7.5k stars, active commits.

### On round-tripping

An earlier draft of this document argued for wire strings partly on the grounds
that reformatting a decimal would break the request signature, since request
bodies are covered by a SHA-256 digest. **That reasoning was wrong.** The digest
is computed over the bytes actually transmitted, so a reformatted value is
self-consistent and verifies correctly.

The real divergence is narrower: shopspring normalises trailing zeros, so `1.50`
marshals as `1.5`. Those are mathematically identical, and no evidence suggests
Webull cares about the textual form. Worth knowing, not worth designing around.

### Distinguishing absent from zero

§23 requires that omission be distinguishable from a zero value, which matters
acutely for prices — a market order has no limit price, and a limit price of zero
is a different and dangerous claim.

The plain type cannot express this. Measured behaviour:

| JSON input | `decimal.Decimal` | `decimal.NullDecimal` | `*decimal.Decimal` |
|---|---|---|---|
| `{"price":"1.5"}` | `"1.5"` | `Valid=true`, `"1.5"` | set |
| `{"price":null}` | `"0"` | `Valid=false` | nil |
| `{}` — omitted | `"0"` | `Valid=false` | nil |

A zero-value `decimal.Decimal` stringifies as `"0"` and reports `IsZero() == true`,
so "the server sent zero" and "the server sent nothing" are indistinguishable.

`NullDecimal` and pointers both solve it, but they serialise differently when
unset, and that difference decides where each belongs:

- `NullDecimal` emits `{"price":null}` — an explicit null.
- `*decimal.Decimal` with `omitempty` emits `{}` — the field is absent.

**Therefore:**

| | Optional field | Always-present field |
|---|---|---|
| **Response models** | `decimal.NullDecimal` | `decimal.Decimal` |
| **Request models** | `*decimal.Decimal` with `omitempty` | `decimal.Decimal` |

This split is principled rather than inconsistent. On responses, `NullDecimal`
is strictly better than a pointer: `resp.Price.Decimal` is always safe to call,
whereas a nil `*decimal.Decimal` panics when dereferenced — and the read path is
the one users touch most. On requests, sending `"limit_price": null` is not
equivalent to omitting the field, and APIs that accept an absent optional
parameter may reject an explicit null for it. Omitting is the conservative
default.

Which of those Webull actually accepts is unverified and is in the sandbox
validation backlog.

### Performance note

shopspring is `big.Int`-backed and allocates. On a high-rate MQTT tick stream,
where every field arrives as a protobuf string, parsing them all into decimals
may show up in profiles.

The right response is to measure before acting. Splitting the public API between
two numeric representations to chase an unmeasured cost would trade a certain
loss in coherence for a speculative gain.

### Status

**This is a decision gate and it has been taken.** Confirmed before any model
code exists, because reversing it later would touch every order, position and
quote type in the SDK.

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
