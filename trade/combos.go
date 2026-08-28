package trade

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/sfreiberg/webull/internal/transport"
)

// Combo is a group of orders placed together: a bracket (an order with
// take-profit and stop-loss orders attached), an OTO, an OCO or an OTOCO.
// Build one with Bracket, OTO, OCO or OTOCO, which assign each order's role,
// and place it with PlaceCombo.
//
// Every order in a group is a complete Order in its own right: it carries its
// own symbol, side, quantity and ClientOrderID, and can be looked up
// individually with Order. Cancelling the RoleMaster cancels the whole group.
//
// Webull's rules, enforced locally before any request:
//
//   - OTO, OCO and OTOCO groups are equity-only.
//   - Brackets on options require a single-leg order; multi-leg strategies
//     cannot be grouped at all.
//   - Each role admits only certain order types and counts; see the
//     constructors.
//
// The sandbox accepts brackets but rejects OTO, OCO and OTOCO groups with
// "invalid combo_type", contrary to Webull's documentation. Those three are
// implemented to the documentation and are unverified.
type Combo struct {
	// ClientComboOrderID identifies the group. If empty, PlaceCombo generates
	// one and writes it back before sending.
	ClientComboOrderID string
	Orders             []*Order
}

// Bracket attaches a take-profit and/or stop-loss order to a master order.
//
// The master is a Market or Limit order. takeProfit is a Limit order and
// stopLoss a RoleStopLoss order, each on the opposite side; either may be nil but
// not both. To attach exits to an existing position, pass a nil master and
// give the exits the closing side.
func Bracket(master, takeProfit, stopLoss *Order) *Combo {
	c := &Combo{}
	c.add(RoleMaster, master)
	c.add(RoleStopProfit, takeProfit)
	c.add(RoleStopLoss, stopLoss)
	return c
}

// add appends o with the given role. A nil order is recorded so that prepare
// reports it rather than a constructor panicking; Bracket alone treats nil as
// "no such order".
func (c *Combo) add(role ComboType, o *Order) {
	if o == nil {
		return
	}
	o.ComboType = role
	c.Orders = append(c.Orders, o)
}

// OTO is a one-triggers-other group: when master fills, the triggered orders
// (one to six) are submitted.
func OTO(master *Order, triggered ...*Order) *Combo {
	c := &Combo{}
	c.addRequired(RoleMaster, master)
	for _, o := range triggered {
		c.addRequired(RoleOTO, o)
	}
	return c
}

// addRequired is add for positions where nil is a caller error: the nil is
// kept so that prepare can report it.
func (c *Combo) addRequired(role ComboType, o *Order) {
	if o == nil {
		c.Orders = append(c.Orders, nil)
		return
	}
	o.ComboType = role
	c.Orders = append(c.Orders, o)
}

// OCO is a one-cancels-other group of two to six orders: when one fills, the
// rest are cancelled.
func OCO(orders ...*Order) *Combo {
	c := &Combo{}
	for _, o := range orders {
		c.addRequired(RoleOCO, o)
	}
	return c
}

// OTOCO is a one-triggers-OCO group: when master fills, the given orders (one
// to six) are submitted as an OCO set.
func OTOCO(master *Order, oco ...*Order) *Combo {
	c := &Combo{}
	c.addRequired(RoleMaster, master)
	for _, o := range oco {
		c.addRequired(RoleOTOCO, o)
	}
	return c
}

// roleRule is what Webull permits for one role within one kind of group.
type roleRule struct {
	types    []OrderType
	min, max int
}

func (r roleRule) allows(t OrderType) bool {
	for _, x := range r.types {
		if x == t {
			return true
		}
	}
	return false
}

var (
	marketOrLimit = []OrderType{Market, Limit}
	triggerable   = []OrderType{Market, Limit, StopLoss, StopLossLimit}
	cancellable   = []OrderType{Limit, StopLoss, StopLossLimit}

	// comboRules is Webull's table of allowed roles per group kind, keyed by
	// the role that identifies the kind.
	comboRules = map[ComboType]map[ComboType]roleRule{
		RoleStopProfit: { // bracket
			RoleMaster:     {marketOrLimit, 0, 1},
			RoleStopProfit: {[]OrderType{Limit}, 0, 1},
			RoleStopLoss:   {[]OrderType{StopLoss}, 0, 1},
		},
		RoleOTO: {
			RoleMaster: {triggerable, 1, 1},
			RoleOTO:    {triggerable, 1, 6},
		},
		RoleOCO: {
			RoleOCO: {cancellable, 2, 6},
		},
		RoleOTOCO: {
			RoleMaster: {triggerable, 1, 1},
			RoleOTOCO:  {cancellable, 1, 6},
		},
	}
)

// kind identifies which group the roles present describe.
func (c *Combo) kind() (ComboType, error) {
	roles := map[ComboType]bool{}
	for _, o := range c.Orders {
		if o != nil {
			roles[o.ComboType] = true
		}
	}
	switch {
	case roles[RoleOTOCO]:
		return RoleOTOCO, nil
	case roles[RoleOTO]:
		return RoleOTO, nil
	case roles[RoleOCO]:
		return RoleOCO, nil
	case roles[RoleStopProfit] || roles[RoleStopLoss]:
		return RoleStopProfit, nil
	default:
		return "", fmt.Errorf("%w: a combo needs at least one dependent order; use Bracket, OTO, OCO or OTOCO", ErrInvalidOrder)
	}
}

// prepare validates the group and every order in it.
func (c *Combo) prepare() error {
	if len(c.Orders) == 0 {
		return fmt.Errorf("%w: empty combo", ErrInvalidOrder)
	}
	for i, o := range c.Orders {
		if o == nil {
			return fmt.Errorf("%w: order %d is nil", ErrInvalidOrder, i)
		}
	}
	if c.ClientComboOrderID == "" {
		c.ClientComboOrderID = newClientOrderID()
	}
	if n := len(c.ClientComboOrderID); n < clientOrderIDMin || n > clientOrderIDMax {
		return fmt.Errorf("%w: ClientComboOrderID must be %d to %d characters, got %d",
			ErrInvalidOrder, clientOrderIDMin, clientOrderIDMax, n)
	}
	kind, err := c.kind()
	if err != nil {
		return err
	}
	rules := comboRules[kind]

	var problems []string
	counts := map[ComboType]int{}
	ids := map[string]bool{}
	for i, o := range c.Orders {
		if err := o.prepare(); err != nil {
			return fmt.Errorf("order %d: %w", i, err)
		}
		if ids[o.ClientOrderID] {
			problems = append(problems, fmt.Sprintf("order %d reuses ClientOrderID %q", i, o.ClientOrderID))
		}
		ids[o.ClientOrderID] = true

		rule, ok := rules[o.ComboType]
		if !ok {
			problems = append(problems, fmt.Sprintf("order %d: role %s does not belong in a %s group", i, o.ComboType, kind))
			continue
		}
		counts[o.ComboType]++
		if !rule.allows(o.Type) {
			problems = append(problems, fmt.Sprintf("order %d: %s is not allowed for role %s", i, o.Type, o.ComboType))
		}
		switch kind {
		case RoleOTO, RoleOCO, RoleOTOCO:
			if o.InstrumentType != InstrumentEquity {
				problems = append(problems, fmt.Sprintf("order %d: %s groups are equity-only", i, kind))
			}
		default:
			if o.InstrumentType == InstrumentOption && o.OptionStrategy != StrategySingle {
				problems = append(problems, fmt.Sprintf("order %d: multi-leg option orders cannot be grouped", i))
			}
		}
	}
	for role, rule := range rules {
		if n := counts[role]; n < rule.min || n > rule.max {
			problems = append(problems, fmt.Sprintf("%s group needs %d to %d %s orders, got %d", kind, rule.min, rule.max, role, n))
		}
	}
	if kind == RoleStopProfit {
		if counts[RoleStopProfit]+counts[RoleStopLoss] == 0 {
			problems = append(problems, "a bracket needs a take-profit or a stop-loss order")
		}
		// Exits close what the master opens: same symbol, opposite side.
		var master *Order
		for _, o := range c.Orders {
			if o.ComboType == RoleMaster {
				master = o
			}
		}
		if master != nil {
			for i, o := range c.Orders {
				if o.ComboType == RoleMaster {
					continue
				}
				if o.Symbol != master.Symbol {
					problems = append(problems, fmt.Sprintf("order %d: exit symbol %s does not match the master's %s", i, o.Symbol, master.Symbol))
				}
				if !opposite(master.Side, o.Side) {
					problems = append(problems, fmt.Sprintf("order %d: exit side %s must be opposite to the master's %s", i, o.Side, master.Side))
				}
			}
		}
	}
	if len(problems) > 0 {
		return fmt.Errorf("%w: %s", ErrInvalidOrder, strings.Join(problems, "; "))
	}
	return nil
}

type comboEnvelope struct {
	AccountID          string  `json:"account_id"`
	ClientComboOrderID string  `json:"client_combo_order_id"`
	NewOrders          []Order `json:"new_orders"`
}

// envelope builds the request body. Preview keeps the preview-only fields;
// placement strips them, as PlaceOrder does.
func (c *Combo) envelope(accountID string, placing bool) comboEnvelope {
	env := comboEnvelope{AccountID: accountID, ClientComboOrderID: c.ClientComboOrderID}
	for _, o := range c.Orders {
		if placing {
			env.NewOrders = append(env.NewOrders, o.forPlacement())
		} else {
			env.NewOrders = append(env.NewOrders, *o)
		}
	}
	return env
}

// opposite reports whether b closes a position opened by a.
func opposite(a, b Side) bool {
	switch a {
	case Buy:
		return b == Sell
	case Sell, Short:
		return b == Buy
	}
	return false
}

// PreviewCombo estimates the cost of a group without placing it.
func (c *Client) PreviewCombo(ctx context.Context, accountID string, combo *Combo) (*OrderPreview, error) {
	if err := combo.prepare(); err != nil {
		return nil, err
	}
	var out OrderPreview
	if err := c.post(ctx, "/trading/orders/preview", combo.envelope(accountID, false), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// PlaceCombo submits a group. The receipt carries the group's ComboOrderID
// and ClientComboOrderID; the individual orders are identified by their own
// ClientOrderIDs, which are generated and written back before sending. Like
// PlaceOrder it never retries.
func (c *Client) PlaceCombo(ctx context.Context, accountID string, combo *Combo) (*OrderReceipt, error) {
	if err := combo.prepare(); err != nil {
		return nil, err
	}
	var out OrderReceipt
	if err := c.post(ctx, "/trading/orders/place", combo.envelope(accountID, true), &out); err != nil {
		return nil, classify(err)
	}
	return &out, nil
}

// CancelCombo cancels every order in a group that can still be cancelled.
//
// Cancelling an unfilled master cancels its exits too, but once the master
// has filled the exits are working orders in their own right, so every order
// is attempted. Orders that are already gone — cancelled along with a sibling,
// filled, or otherwise not cancellable — are not errors: the group's goal is
// that nothing is left working. Any other failure is returned, joined, after
// every order has been attempted.
func (c *Client) CancelCombo(ctx context.Context, accountID string, combo *Combo) error {
	if combo == nil || len(combo.Orders) == 0 {
		return fmt.Errorf("%w: empty combo", ErrInvalidOrder)
	}
	var errs []error
	for _, o := range combo.Orders {
		if o == nil {
			continue
		}
		if _, err := c.CancelOrder(ctx, accountID, o.ClientOrderID); err != nil && !alreadyDone(err) {
			errs = append(errs, fmt.Errorf("%s %s: %w", o.ComboType, o.ClientOrderID, err))
		}
	}
	return errors.Join(errs...)
}

// alreadyDone reports whether a cancel failed because there was nothing left
// to cancel.
func alreadyDone(err error) bool {
	return transport.HasCode(err, "OPENAPI_ORDER_NOT_FOUND", "OPENAPI_ORDER_CAN_NOT_BE_CANCEL")
}
