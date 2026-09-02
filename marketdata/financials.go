package marketdata

import (
	"context"

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

	TotalAssets                 NullDecimal `json:"total_assets"`
	TotalCurrentAssets          NullDecimal `json:"total_cur_assets"`
	CashAndShortTermInvest      NullDecimal `json:"cash_st_invest"`
	Cash                        NullDecimal `json:"cash"`
	CashEquivalents             NullDecimal `json:"cash_equiv"`
	ShortTermInvestments        NullDecimal `json:"st_invest"`
	TotalReceivablesNet         NullDecimal `json:"total_recv_net"`
	TradeReceivablesNet         NullDecimal `json:"ar_trade_net"`
	TotalInventory              NullDecimal `json:"total_inv"`
	PrepaidExpenses             NullDecimal `json:"prepaid_expenses"`
	OtherCurrentAssets          NullDecimal `json:"other_cur_assets"`
	TotalNonCurrentAssets       NullDecimal `json:"total_non_cur_assets"`
	PPENet                      NullDecimal `json:"ppe_net"`
	PPEGross                    NullDecimal `json:"ppe_gross"`
	AccumulatedDepreciation     NullDecimal `json:"acc_depre"`
	GoodwillNet                 NullDecimal `json:"goodwill_net"`
	IntangiblesNet              NullDecimal `json:"intangibles_net"`
	LongTermInvestments         NullDecimal `json:"lt_invest"`
	LongTermNotesReceivable     NullDecimal `json:"note_rece_long_term"`
	OtherLongTermAssets         NullDecimal `json:"other_lt_assets"`
	TotalLiabilities            NullDecimal `json:"total_liab"`
	TotalCurrentLiabilities     NullDecimal `json:"total_cur_liab"`
	AccountsPayable             NullDecimal `json:"ap"`
	AccruedExpenses             NullDecimal `json:"accrued_expenses"`
	ShortTermDebt               NullDecimal `json:"notes_st_debt"`
	CurrentLongTermDebt         NullDecimal `json:"cur_lt_debt_lease"`
	OtherCurrentLiabilities     NullDecimal `json:"other_cur_liab"`
	TotalNonCurrentLiabilities  NullDecimal `json:"total_non_cur_liab"`
	TotalLongTermDebt           NullDecimal `json:"total_lt_debt"`
	LongTermDebt                NullDecimal `json:"lt_debt"`
	CapitalLeaseObligations     NullDecimal `json:"capital_lease_obligations"`
	TotalDebt                   NullDecimal `json:"total_debt"`
	OtherLiabilities            NullDecimal `json:"other_liab"`
	MinorityInterest            NullDecimal `json:"minority_interest"`
	TotalEquity                 NullDecimal `json:"total_equity"`
	TotalShareholdersEquity     NullDecimal `json:"total_sh_equity"`
	NonRedeemablePreferredStock NullDecimal `json:"non_redeemable_preferred_stock"`
	CommonStock                 NullDecimal `json:"common_stock"`
	AdditionalPaidInCapital     NullDecimal `json:"apic"`
	RetainedEarnings            NullDecimal `json:"retained_earnings"`
	OtherEquity                 NullDecimal `json:"other_equity"`
	TotalLiabilitiesAndEquity   NullDecimal `json:"total_liab_sh_equity"`
	CommonSharesOutstanding     NullDecimal `json:"common_shares_out"`
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

	TotalRevenue             NullDecimal `json:"total_revenue"`
	Revenue                  NullDecimal `json:"revenue"`
	CostOfRevenue            NullDecimal `json:"cost_of_revenue"`
	GrossProfit              NullDecimal `json:"gross_profit"`
	OperatingExpenses        NullDecimal `json:"opex"`
	SGAExpenses              NullDecimal `json:"sga_exp"`
	RnDExpenses              NullDecimal `json:"rnd_exp"`
	UnusualExpenseIncome     NullDecimal `json:"unusual_expense_income"`
	OperatingIncome          NullDecimal `json:"op_income"`
	OperatingProfit          NullDecimal `json:"op_profit"`
	NonOperatingInterestNet  NullDecimal `json:"inter_inc_expse_net_non_oper"`
	GainOnSaleOfAssets       NullDecimal `json:"gain_loss_on_sale_of_assets"`
	OtherNetIncome           NullDecimal `json:"other_net_income"`
	PretaxIncome             NullDecimal `json:"ebt"`
	PretaxEarnings           NullDecimal `json:"ebt_alt"`
	IncomeTax                NullDecimal `json:"income_tax"`
	AfterTaxIncome           NullDecimal `json:"eat"`
	AfterTaxEarnings         NullDecimal `json:"eat_alt"`
	MinorityInterest         NullDecimal `json:"minority_interest"`
	IncomeBeforeExtraItems   NullDecimal `json:"ni_pre_extra"`
	ExtraordinaryItems       NullDecimal `json:"extra_items"`
	NetIncome                NullDecimal `json:"net_income"`
	NetIncomeCommonExclExtra NullDecimal `json:"ni_common_excl_extra"`
	NetIncomeCommonInclExtra NullDecimal `json:"ni_common_incl_extra"`
	DilutedNetIncome         NullDecimal `json:"diluted_ni"`
	DilutedAvgShares         NullDecimal `json:"diluted_avg_shares"`
	DilutedEPSExclExtra      NullDecimal `json:"diluted_eps_excl_extra"`
	DilutedEPSInclExtra      NullDecimal `json:"diluted_eps_incl_extra"`
	DilutedNormalizedEPS     NullDecimal `json:"diluted_norm_eps"`
	DividendsPerShare        NullDecimal `json:"dps"`
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

	OperatingCashFlow        NullDecimal `json:"cfo"`
	NetIncome                NullDecimal `json:"net_income"`
	DepreciationAmortization NullDecimal `json:"dna"`
	DeferredTaxes            NullDecimal `json:"deferred_tax"`
	NonCashItems             NullDecimal `json:"non_cash_items"`
	WorkingCapitalChange     NullDecimal `json:"wc_change"`
	InvestingCashFlow        NullDecimal `json:"cfi"`
	CapitalExpenditures      NullDecimal `json:"capex"`
	OtherInvestingItems      NullDecimal `json:"other_cfi_items"`
	FinancingCashFlow        NullDecimal `json:"cff"`
	FinancingItems           NullDecimal `json:"cff_items"`
	NetStockIssuance         NullDecimal `json:"net_stock_iss_ret"`
	NetDebtIssuance          NullDecimal `json:"net_debt_iss_ret"`
	FXEffects                NullDecimal `json:"fx_effects"`
	NetChangeInCash          NullDecimal `json:"net_change_cash"`
	InterestPaid             NullDecimal `json:"interest_paid"`
	TaxesPaid                NullDecimal `json:"taxes_paid"`
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
	FiscalPeriod int         `json:"fiscal_period"`
	Value        NullDecimal `json:"value"`
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
	FiscalPeriod    int         `json:"fiscal_period"`
	Currency        string      `json:"currency"`
	EPSEstimate     NullDecimal `json:"eps_est"`
	EPSLastYear     NullDecimal `json:"eps_ly"`
	RevenueEstimate NullDecimal `json:"rev_est"`
	RevenueLastYear NullDecimal `json:"rev_ly"`
}

// FinancialAlert returns the estimate for a company's next financial report.
func (c *Client) FinancialAlert(ctx context.Context, symbol string, cat Category) (*FinancialAlert, error) {
	var out FinancialAlert
	if err := c.get(ctx, "/market-data/fundamentals/financial-alerts/get", symbolParams(symbol, cat), &out); err != nil {
		return nil, err
	}
	return &out, nil
}
