package marketdata

import (
	"context"

	"github.com/shopspring/decimal"

	"github.com/sfreiberg/webull/internal/query"
)

// FinancialType selects annual or quarterly financial reports.
type FinancialType string

// Financial report types.
const (
	Annual    FinancialType = "ANNUAL"
	Quarterly FinancialType = "QUARTERLY"
)

// FinancialsRequest selects a company's financial reports.
type FinancialsRequest struct {
	Symbol string
	// Category defaults to USStock, the only documented value.
	Category Category
	// Type is Annual or Quarterly; empty lets Webull default to Quarterly.
	Type FinancialType
	// Count is between 1 and 20; zero lets Webull apply its default of 5.
	Count int
}

func (r FinancialsRequest) params() query.Params {
	q := symbolParams(r.Symbol, r.Category)
	q.Set("type", string(r.Type))
	q.SetInt("count", r.Count)
	return q
}

// ReportPeriod identifies the reporting period of a financial statement.
type ReportPeriod struct {
	FiscalYear int `json:"fiscal_year"`
	// FiscalPeriod is 0 for the full year and 1 through 4 for quarters.
	FiscalPeriod int    `json:"fiscal_period"`
	EndDate      Time   `json:"end_date"`
	PublishDate  Time   `json:"publish_date"`
	Currency     string `json:"currency"`
}

// BalanceSheet is one reporting period's balance sheet. Line items a company
// does not report are null.
type BalanceSheet struct {
	ReportPeriod

	TotalAssets                 decimal.NullDecimal `json:"total_assets"`
	TotalCurrentAssets          decimal.NullDecimal `json:"total_cur_assets"`
	CashAndShortTermInvest      decimal.NullDecimal `json:"cash_st_invest"`
	Cash                        decimal.NullDecimal `json:"cash"`
	CashEquivalents             decimal.NullDecimal `json:"cash_equiv"`
	ShortTermInvestments        decimal.NullDecimal `json:"st_invest"`
	TotalReceivablesNet         decimal.NullDecimal `json:"total_recv_net"`
	TradeReceivablesNet         decimal.NullDecimal `json:"ar_trade_net"`
	TotalInventory              decimal.NullDecimal `json:"total_inv"`
	PrepaidExpenses             decimal.NullDecimal `json:"prepaid_expenses"`
	OtherCurrentAssets          decimal.NullDecimal `json:"other_cur_assets"`
	TotalNonCurrentAssets       decimal.NullDecimal `json:"total_non_cur_assets"`
	PPENet                      decimal.NullDecimal `json:"ppe_net"`
	PPEGross                    decimal.NullDecimal `json:"ppe_gross"`
	AccumulatedDepreciation     decimal.NullDecimal `json:"acc_depre"`
	GoodwillNet                 decimal.NullDecimal `json:"goodwill_net"`
	IntangiblesNet              decimal.NullDecimal `json:"intangibles_net"`
	LongTermInvestments         decimal.NullDecimal `json:"lt_invest"`
	LongTermNotesReceivable     decimal.NullDecimal `json:"note_rece_long_term"`
	OtherLongTermAssets         decimal.NullDecimal `json:"other_lt_assets"`
	TotalLiabilities            decimal.NullDecimal `json:"total_liab"`
	TotalCurrentLiabilities     decimal.NullDecimal `json:"total_cur_liab"`
	AccountsPayable             decimal.NullDecimal `json:"ap"`
	AccruedExpenses             decimal.NullDecimal `json:"accrued_expenses"`
	ShortTermDebt               decimal.NullDecimal `json:"notes_st_debt"`
	CurrentLongTermDebt         decimal.NullDecimal `json:"cur_lt_debt_lease"`
	OtherCurrentLiabilities     decimal.NullDecimal `json:"other_cur_liab"`
	TotalNonCurrentLiabilities  decimal.NullDecimal `json:"total_non_cur_liab"`
	TotalLongTermDebt           decimal.NullDecimal `json:"total_lt_debt"`
	LongTermDebt                decimal.NullDecimal `json:"lt_debt"`
	CapitalLeaseObligations     decimal.NullDecimal `json:"capital_lease_obligations"`
	TotalDebt                   decimal.NullDecimal `json:"total_debt"`
	OtherLiabilities            decimal.NullDecimal `json:"other_liab"`
	MinorityInterest            decimal.NullDecimal `json:"minority_interest"`
	TotalEquity                 decimal.NullDecimal `json:"total_equity"`
	TotalShareholdersEquity     decimal.NullDecimal `json:"total_sh_equity"`
	NonRedeemablePreferredStock decimal.NullDecimal `json:"non_redeemable_preferred_stock"`
	CommonStock                 decimal.NullDecimal `json:"common_stock"`
	AdditionalPaidInCapital     decimal.NullDecimal `json:"apic"`
	RetainedEarnings            decimal.NullDecimal `json:"retained_earnings"`
	OtherEquity                 decimal.NullDecimal `json:"other_equity"`
	TotalLiabilitiesAndEquity   decimal.NullDecimal `json:"total_liab_sh_equity"`
	CommonSharesOutstanding     decimal.NullDecimal `json:"common_shares_out"`
}

// BalanceSheets returns a company's balance sheets, most recent first.
func (c *Client) BalanceSheets(ctx context.Context, req FinancialsRequest) ([]BalanceSheet, error) {
	var out []BalanceSheet
	if err := c.get(ctx, "/market-data/fundamentals/balance-sheets/get", req.params(), &out); err != nil {
		return nil, err
	}
	return out, nil
}

// IncomeStatement is one reporting period's income statement. Line items a
// company does not report are null.
type IncomeStatement struct {
	ReportPeriod

	TotalRevenue             decimal.NullDecimal `json:"total_revenue"`
	Revenue                  decimal.NullDecimal `json:"revenue"`
	CostOfRevenue            decimal.NullDecimal `json:"cost_of_revenue"`
	GrossProfit              decimal.NullDecimal `json:"gross_profit"`
	OperatingExpenses        decimal.NullDecimal `json:"opex"`
	SGAExpenses              decimal.NullDecimal `json:"sga_exp"`
	RnDExpenses              decimal.NullDecimal `json:"rnd_exp"`
	UnusualExpenseIncome     decimal.NullDecimal `json:"unusual_expense_income"`
	OperatingIncome          decimal.NullDecimal `json:"op_income"`
	OperatingProfit          decimal.NullDecimal `json:"op_profit"`
	NonOperatingInterestNet  decimal.NullDecimal `json:"inter_inc_expse_net_non_oper"`
	GainOnSaleOfAssets       decimal.NullDecimal `json:"gain_loss_on_sale_of_assets"`
	OtherNetIncome           decimal.NullDecimal `json:"other_net_income"`
	PretaxIncome             decimal.NullDecimal `json:"ebt"`
	PretaxEarnings           decimal.NullDecimal `json:"ebt_alt"`
	IncomeTax                decimal.NullDecimal `json:"income_tax"`
	AfterTaxIncome           decimal.NullDecimal `json:"eat"`
	AfterTaxEarnings         decimal.NullDecimal `json:"eat_alt"`
	MinorityInterest         decimal.NullDecimal `json:"minority_interest"`
	IncomeBeforeExtraItems   decimal.NullDecimal `json:"ni_pre_extra"`
	ExtraordinaryItems       decimal.NullDecimal `json:"extra_items"`
	NetIncome                decimal.NullDecimal `json:"net_income"`
	NetIncomeCommonExclExtra decimal.NullDecimal `json:"ni_common_excl_extra"`
	NetIncomeCommonInclExtra decimal.NullDecimal `json:"ni_common_incl_extra"`
	DilutedNetIncome         decimal.NullDecimal `json:"diluted_ni"`
	DilutedAvgShares         decimal.NullDecimal `json:"diluted_avg_shares"`
	DilutedEPSExclExtra      decimal.NullDecimal `json:"diluted_eps_excl_extra"`
	DilutedEPSInclExtra      decimal.NullDecimal `json:"diluted_eps_incl_extra"`
	DilutedNormalizedEPS     decimal.NullDecimal `json:"diluted_norm_eps"`
	DividendsPerShare        decimal.NullDecimal `json:"dps"`
}

// IncomeStatements returns a company's income statements, most recent first.
func (c *Client) IncomeStatements(ctx context.Context, req FinancialsRequest) ([]IncomeStatement, error) {
	var out []IncomeStatement
	if err := c.get(ctx, "/market-data/fundamentals/income-statements/get", req.params(), &out); err != nil {
		return nil, err
	}
	return out, nil
}

// CashFlow is one reporting period's cash flow statement. Line items a
// company does not report are null.
type CashFlow struct {
	ReportPeriod

	OperatingCashFlow        decimal.NullDecimal `json:"cfo"`
	NetIncome                decimal.NullDecimal `json:"net_income"`
	DepreciationAmortization decimal.NullDecimal `json:"dna"`
	DeferredTaxes            decimal.NullDecimal `json:"deferred_tax"`
	NonCashItems             decimal.NullDecimal `json:"non_cash_items"`
	WorkingCapitalChange     decimal.NullDecimal `json:"wc_change"`
	InvestingCashFlow        decimal.NullDecimal `json:"cfi"`
	CapitalExpenditures      decimal.NullDecimal `json:"capex"`
	OtherInvestingItems      decimal.NullDecimal `json:"other_cfi_items"`
	FinancingCashFlow        decimal.NullDecimal `json:"cff"`
	FinancingItems           decimal.NullDecimal `json:"cff_items"`
	NetStockIssuance         decimal.NullDecimal `json:"net_stock_iss_ret"`
	NetDebtIssuance          decimal.NullDecimal `json:"net_debt_iss_ret"`
	FXEffects                decimal.NullDecimal `json:"fx_effects"`
	NetChangeInCash          decimal.NullDecimal `json:"net_change_cash"`
	InterestPaid             decimal.NullDecimal `json:"interest_paid"`
	TaxesPaid                decimal.NullDecimal `json:"taxes_paid"`
}

// CashFlows returns a company's cash flow statements, most recent first.
func (c *Client) CashFlows(ctx context.Context, req FinancialsRequest) ([]CashFlow, error) {
	var out []CashFlow
	if err := c.get(ctx, "/market-data/fundamentals/cash-flows/get", req.params(), &out); err != nil {
		return nil, err
	}
	return out, nil
}

// IndicatorValue is one reporting period's value of a financial indicator.
// Value is null when Webull has no figure for the period.
type IndicatorValue struct {
	FiscalYear int `json:"fiscal_year"`
	// FiscalPeriod is 0 for the full year and 1 through 4 for quarters.
	FiscalPeriod int                 `json:"fiscal_period"`
	Value        decimal.NullDecimal `json:"value"`
}

// FinancialIndicators holds per-period financial ratios keyed by indicator
// name. The documented keys are roa, roe, diluted_eps_incl_extra, net_margin,
// debt_to_assets, naps, ocf_ps and cap_surplus_ps.
type FinancialIndicators struct {
	Currency string                      `json:"currency"`
	Values   map[string][]IndicatorValue `json:"values"`
}

// FinancialIndicators returns a company's financial ratios per reporting
// period.
func (c *Client) FinancialIndicators(ctx context.Context, req FinancialsRequest) (*FinancialIndicators, error) {
	var out FinancialIndicators
	if err := c.get(ctx, "/market-data/fundamentals/indicators/get", req.params(), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// FinancialAlert is the estimate for a company's next financial report and
// the window in which it is expected.
type FinancialAlert struct {
	// StartDate and EndDate bound the expected publication window.
	StartDate  Time `json:"start_date"`
	EndDate    Time `json:"end_date"`
	FiscalYear int  `json:"fiscal_year"`
	// FiscalPeriod is 1 through 4 for quarters, or 5 for a pre-release.
	FiscalPeriod    int                 `json:"fiscal_period"`
	Currency        string              `json:"currency"`
	EPSEstimate     decimal.NullDecimal `json:"eps_est"`
	EPSLastYear     decimal.NullDecimal `json:"eps_ly"`
	RevenueEstimate decimal.NullDecimal `json:"rev_est"`
	RevenueLastYear decimal.NullDecimal `json:"rev_ly"`
}

// FinancialAlert returns the estimate for a company's next financial report.
func (c *Client) FinancialAlert(ctx context.Context, symbol string, cat Category) (*FinancialAlert, error) {
	var out FinancialAlert
	if err := c.get(ctx, "/market-data/fundamentals/financial-alerts/get", symbolParams(symbol, cat), &out); err != nil {
		return nil, err
	}
	return &out, nil
}
