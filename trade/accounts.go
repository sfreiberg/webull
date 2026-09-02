package trade

import (
	"context"
	"time"

	"github.com/sfreiberg/webull/internal/query"
)

// Account is a brokerage account accessible to the API key.
type Account struct {
	AccountID     string       `json:"account_id"`
	AccountNumber string       `json:"account_number"`
	AccountType   AccountType  `json:"account_type"`
	AccountLabel  string       `json:"account_label"`
	AccountClass  AccountClass `json:"account_class"`
	UserID        string       `json:"user_id"`
}

// Accounts lists the accounts this API key can access.
func (c *Client) Accounts(ctx context.Context) ([]Account, error) {
	var out []Account
	if err := c.get(ctx, "/trading/accounts/list", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Balance is the financial summary of an account.
//
// Fields documented as always present are Decimal. The rest are NullDecimal:
// Webull omits them for some account classes, and an omitted margin figure is
// not the same thing as a margin figure of zero.
type Balance struct {
	Currency                  string  `json:"total_asset_currency"`
	TotalCashBalance          Decimal `json:"total_cash_balance"`
	TotalMarketValue          Decimal `json:"total_market_value"`
	TotalUnrealizedProfitLoss Decimal `json:"total_unrealized_profit_loss"`
	TotalNetLiquidationValue  Decimal `json:"total_net_liquidation_value"`
	TotalDayProfitLoss        Decimal `json:"total_day_profit_loss"`

	// DayTradesLeft is a count, or "UNLIMITED". It is left as a string because
	// the sentinel makes it neither a number nor an enumeration.
	DayTradesLeft string `json:"day_trades_left"`

	MaintenanceMargin      NullDecimal `json:"maintenance_margin"`
	UsedMargin             NullDecimal `json:"used_margin"`
	UsedMarginForOpenOrder NullDecimal `json:"used_margin_for_open_order"`
	InitMargin             NullDecimal `json:"init_margin"`
	IntradayMargin         NullDecimal `json:"intraday_margin"`
	MarginExcess           NullDecimal `json:"margin_excess"`
	MarginRatio            NullDecimal `json:"margin_ratio"`

	// OpenMarginCalls lists outstanding margin calls. Webull's documentation
	// describes this as a single value; the API returns a list.
	OpenMarginCalls []MarginCall `json:"open_margin_calls"`

	CurrencyAssets []CurrencyAssets `json:"account_currency_assets"`
}

// CurrencyAssets is the per-currency breakdown of a Balance.
type CurrencyAssets struct {
	Currency             string  `json:"currency"`
	CashBalance          Decimal `json:"cash_balance"`
	MarketValue          Decimal `json:"market_value"`
	UnrealizedProfitLoss Decimal `json:"unrealized_profit_loss"`

	SettledCash             NullDecimal `json:"settled_cash"`
	UnsettledCash           NullDecimal `json:"unsettled_cash"`
	HeldAmount              NullDecimal `json:"held_amount"`
	FrozenAmount            NullDecimal `json:"frozen_amount"`
	BuyingPower             NullDecimal `json:"buying_power"`
	AvailableWithdrawal     NullDecimal `json:"available_withdrawal"`
	InterestsUnpaid         NullDecimal `json:"interests_unpaid"`
	NetLiquidationValue     NullDecimal `json:"net_liquidation_value"`
	OptionBuyingPower       NullDecimal `json:"option_buying_power"`
	DayBuyingPower          NullDecimal `json:"day_buying_power"`
	OvernightBuyingPower    NullDecimal `json:"overnight_buying_power"`
	NightTradingBuyingPower NullDecimal `json:"night_trading_buying_power"`
	DayProfitLoss           NullDecimal `json:"day_profit_loss"`
	UsedMargin              NullDecimal `json:"used_margin"`
	UsedMarginForOpenOrder  NullDecimal `json:"used_margin_for_open_order"`
	InitMargin              NullDecimal `json:"init_margin"`
	MaintenanceMargin       NullDecimal `json:"maintenance_margin"`
	IntradayMargin          NullDecimal `json:"intraday_margin"`
	MarginExcess            NullDecimal `json:"margin_excess"`
	MarginRatio             NullDecimal `json:"margin_ratio"`
}

// Balance returns the balance of an account.
func (c *Client) Balance(ctx context.Context, accountID string) (*Balance, error) {
	q := query.New()
	q.Set("account_id", accountID)
	var out Balance
	if err := c.get(ctx, "/trading/assets/balances/get", q, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Position is an open holding in an account.
type Position struct {
	PositionID           string         `json:"position_id"`
	Currency             string         `json:"currency"`
	Symbol               string         `json:"symbol"`
	InstrumentType       InstrumentType `json:"instrument_type"`
	OptionStrategy       OptionStrategy `json:"option_strategy"`
	Quantity             Decimal        `json:"quantity"`
	LastPrice            Decimal        `json:"last_price"`
	CostPrice            Decimal        `json:"cost_price"`
	UnrealizedProfitLoss Decimal        `json:"unrealized_profit_loss"`

	// EventOutcome is set only for event contract positions.
	EventOutcome EventOutcome `json:"event_outcome"`

	// Legs is populated for option positions.
	Legs []PositionLeg `json:"legs"`
}

// PositionLeg is one leg of an option position.
type PositionLeg struct {
	LegID    string      `json:"leg_id"`
	Symbol   string      `json:"symbol"`
	Quantity NullDecimal `json:"quantity"`

	OptionType OptionType `json:"option_type"`
	// ExpireDate is in yyyy-MM-dd form.
	ExpireDate          string      `json:"option_expire_date"`
	ExercisePrice       NullDecimal `json:"option_exercise_price"`
	ContractMultiplier  NullDecimal `json:"option_contract_multiplier"`
	ContractDeliverable NullDecimal `json:"option_contract_deliverable"`
	ExpirationType      string      `json:"expiration_type"`
}

// Positions lists the open positions in an account. Webull does not paginate
// this endpoint.
func (c *Client) Positions(ctx context.Context, accountID string) ([]Position, error) {
	q := query.New()
	q.Set("account_id", accountID)
	var out []Position
	if err := c.get(ctx, "/trading/assets/positions/list", q, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// CashActivity is a cash movement on an account: a trade, transfer, fee or
// similar.
type CashActivity struct {
	ID              string          `json:"id"`
	AccountID       string          `json:"account_id"`
	ActivityType    ActivityType    `json:"activity_type"`
	ActivitySubType ActivitySubType `json:"activity_sub_type"`
	Currency        string          `json:"currency"`
	Market          string          `json:"market"`
	Symbol          string          `json:"symbol"`
	// TradeDate is the accounting date, in yyyy-MM-dd form.
	TradeDate string `json:"trade_date"`
	// NetAmount is positive for a credit and negative for a debit.
	NetAmount Decimal   `json:"net_amount"`
	BizTime   time.Time `json:"biz_time"`
}

// CashActivitiesRequest filters a cash activity query.
type CashActivitiesRequest struct {
	AccountID string
	// ActivityTypes restricts results to the given kinds. Empty means all.
	ActivityTypes []ActivityType
	// StartTime and EndTime bound the query. When both are zero Webull returns
	// the last seven days.
	StartTime time.Time
	EndTime   time.Time
	// LastActivityID is the ID of the final item from the previous page. Leave
	// empty for the first page.
	LastActivityID string
	// PageSize is between 1 and 200. Zero lets Webull apply its default of 10.
	PageSize int
}

// activityTimeLayout is ISO-8601 UTC with milliseconds, which is the form
// Webull documents for these parameters.
const activityTimeLayout = "2006-01-02T15:04:05.000Z"

// CashActivities returns one page of cash activities. To fetch the next page,
// set LastActivityID to the ID of the last item returned.
func (c *Client) CashActivities(ctx context.Context, req CashActivitiesRequest) ([]CashActivity, error) {
	q := query.New()
	q.Set("account_id", req.AccountID)
	types := make([]string, 0, len(req.ActivityTypes))
	for _, t := range req.ActivityTypes {
		types = append(types, string(t))
	}
	q.SetList("activity_types", types)
	if !req.StartTime.IsZero() {
		q.Set("start_time", req.StartTime.UTC().Format(activityTimeLayout))
	}
	if !req.EndTime.IsZero() {
		q.Set("end_time", req.EndTime.UTC().Format(activityTimeLayout))
	}
	q.Set("last_activity_id", req.LastActivityID)
	q.SetInt("page_size", req.PageSize)

	var out []CashActivity
	if err := c.get(ctx, "/trading/activities/cash-activities/list", q, &out); err != nil {
		return nil, err
	}
	return out, nil
}
