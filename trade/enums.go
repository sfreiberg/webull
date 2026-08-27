package trade

// Enumerations are string types with declared constants rather than closed
// sets. A value Webull adds after this SDK was released round-trips intact
// instead of failing to decode.

// AccountType distinguishes cash and margin accounts.
type AccountType string

// Account types.
const (
	AccountTypeCash   AccountType = "CASH"
	AccountTypeMargin AccountType = "MARGIN"
)

// AccountClass is the regulatory and product classification of an account.
type AccountClass string

// Account classes.
const (
	AccountClassIndividualCash        AccountClass = "INDIVIDUAL_CASH"
	AccountClassIndividualMargin      AccountClass = "INDIVIDUAL_MARGIN"
	AccountClassRothIRA               AccountClass = "ROTH_IRA"
	AccountClassTraditionalIRA        AccountClass = "TRADITIONAL_IRA"
	AccountClassRolloverIRA           AccountClass = "ROLLOVER_IRA"
	AccountClassManagedRothIRA        AccountClass = "MANAGED_ROTH_IRA"
	AccountClassManagedTraditionalIRA AccountClass = "MANAGED_TRADITIONAL_IRA"
	AccountClassCrypto                AccountClass = "CRYPTO"
	AccountClassFutures               AccountClass = "FUTURES"
	AccountClassEventsCash            AccountClass = "EVENTS_CASH"
)

// InstrumentType is the asset class of a position or order.
type InstrumentType string

// Instrument types.
const (
	InstrumentEquity  InstrumentType = "EQUITY"
	InstrumentOption  InstrumentType = "OPTION"
	InstrumentFutures InstrumentType = "FUTURES"
	InstrumentCrypto  InstrumentType = "CRYPTO"
	InstrumentEvent   InstrumentType = "EVENT"
)

// Category identifies a market and asset class in instrument lookups.
type Category string

// Categories.
const (
	CategoryUSStock   Category = "US_STOCK"
	CategoryUSOption  Category = "US_OPTION"
	CategoryUSFutures Category = "US_FUTURES"
	CategoryUSCrypto  Category = "US_CRYPTO"
)

// TradableStatus reports whether an instrument can currently be traded.
type TradableStatus string

// Tradable statuses.
const (
	// Tradable means the instrument may be bought and sold.
	Tradable TradableStatus = "OC"
	// LiquidateOnly means positions may be closed but not opened.
	LiquidateOnly TradableStatus = "CO"
	// NotTradable means no orders are accepted.
	NotTradable TradableStatus = "NT"
)

// OptionType is a call or a put.
type OptionType string

// Option types.
const (
	Call OptionType = "CALL"
	Put  OptionType = "PUT"
)

// OptionStyle is the exercise style of an option contract.
type OptionStyle string

// Option styles.
const (
	American OptionStyle = "AMERICAN"
	European OptionStyle = "EUROPEAN"
)

// OptionStrategy classifies a multi-leg position.
type OptionStrategy string

// Option strategies reported on positions.
const (
	StrategySingle       OptionStrategy = "SINGLE"
	StrategyCoveredStock OptionStrategy = "COVERED_STOCK"
)

// ListingStatus is whether an option contract is currently listed.
type ListingStatus string

// Listing statuses.
const (
	Listing   ListingStatus = "LISTING"
	Delisting ListingStatus = "DELISTING"
)

// StockSubCategory refines an equity instrument.
type StockSubCategory string

// Stock sub-categories.
const (
	CommonStock    StockSubCategory = "COMMON_STOCK"
	ETF            StockSubCategory = "ETF"
	PreferredStock StockSubCategory = "PREFERRED_STOCK"
	Warrant        StockSubCategory = "WARRANT"
	Units          StockSubCategory = "UNITS"
	Right          StockSubCategory = "RIGHT"
)

// ActivityType is the kind of a cash activity.
type ActivityType string

// Activity types.
const (
	ActivityTrade       ActivityType = "TRADE"
	ActivityDeposit     ActivityType = "DEPOSIT"
	ActivityWithdraw    ActivityType = "WITHDRAW"
	ActivityFees        ActivityType = "FEES"
	ActivityJournal     ActivityType = "JOURNAL"
	ActivityInterests   ActivityType = "INTERESTS"
	ActivityECStatement ActivityType = "EC_STATEMENT"
)

// ActivitySubType refines an ActivityType.
type ActivitySubType string

// Activity sub-types.
const (
	SubTypeBuyToOpen    ActivitySubType = "BTO"
	SubTypeSellToClose  ActivitySubType = "STC"
	SubTypeWire         ActivitySubType = "WIRE"
	SubTypeACH          ActivitySubType = "ACH"
	SubTypeWireReverse  ActivitySubType = "WIRE_REVERSE"
	SubTypeACHReverse   ActivitySubType = "ACH_REVERSE"
	SubTypeWireDeposit  ActivitySubType = "WIRE_DEPOSIT"
	SubTypeWireWithdraw ActivitySubType = "WIRE_WITHDRAW"
	SubTypeACHReversal  ActivitySubType = "ACH_REVERSAL"
	SubTypeWireReversal ActivitySubType = "WIRE_REVERSAL"
	SubTypeCashJournal  ActivitySubType = "CASH_JOURNAL"
	SubTypeCredit       ActivitySubType = "CREDIT"
	SubTypeCreditCash   ActivitySubType = "CREDIT_CASH"
	SubTypeECExpiration ActivitySubType = "EC_EXPIRATION"
	SubTypeECPayout     ActivitySubType = "EC_PAYOUT"
	SubTypeOther        ActivitySubType = "OTHER"
)

// MarginCall is a kind of open margin call on an account.
type MarginCall string

// Margin call kinds.
const (
	MarginCallEM MarginCall = "EM"
	MarginCallRM MarginCall = "RM"
	MarginCallRT MarginCall = "RT"
	MarginCallDT MarginCall = "DT"
)

// FuturesContractType distinguishes dated contracts from the continuous one.
type FuturesContractType string

// Futures contract types.
const (
	ContractMonthly FuturesContractType = "MONTHLY"
	ContractMain    FuturesContractType = "MAIN"
)

// Settlement is how a contract settles at expiry.
type Settlement string

// Settlement methods. Webull's documentation shows title case; the live API
// returns upper case. Compare with strings.EqualFold or use Settlement.Is.
const (
	SettlementCash     Settlement = "CASH"
	SettlementPhysical Settlement = "PHYSICAL"
)

// EventCategoryCode identifies a category of event contracts.
type EventCategoryCode string

// Event contract categories.
const (
	EventEconomics      EventCategoryCode = "ECONOMICS"
	EventFinancials     EventCategoryCode = "FINANCIALS"
	EventPolitics       EventCategoryCode = "POLITICS"
	EventEntertainment  EventCategoryCode = "ENTERTAINMENT"
	EventScienceTech    EventCategoryCode = "SCIENCE_TECHNOLOGY"
	EventClimateWeather EventCategoryCode = "CLIMATE_WEATHER"
	EventTransportation EventCategoryCode = "TRANSPORTATION"
	EventCrypto         EventCategoryCode = "CRYPTO"
	EventSports         EventCategoryCode = "SPORTS"
)

// EventFrequency is how often a series recurs.
type EventFrequency string

// Event series frequencies.
const (
	FrequencyHourly  EventFrequency = "HOURLY"
	FrequencyDaily   EventFrequency = "DAILY"
	FrequencyWeekly  EventFrequency = "WEEKLY"
	FrequencyMonthly EventFrequency = "MONTHLY"
	FrequencyAnnual  EventFrequency = "ANNUAL"
	FrequencyOneOff  EventFrequency = "ONE_OFF"
	FrequencyCustom  EventFrequency = "CUSTOM"
)

// EventStatus is whether an event is open.
type EventStatus string

// Event statuses.
const (
	EventActive   EventStatus = "ACTIVE"
	EventInactive EventStatus = "INACTIVE"
)

// EventMarketStatus is the listing status of an event market.
type EventMarketStatus string

// Event market statuses.
const (
	EventMarketNotSet       EventMarketStatus = "NOT_SET"
	EventMarketListing      EventMarketStatus = "LISTING"
	EventMarketDelisting    EventMarketStatus = "DELISTING"
	EventMarketOther        EventMarketStatus = "OTHER"
	EventMarketUnrecognized EventMarketStatus = "UNRECOGNIZED"
)

// EventOutcome is the side of an event contract position.
type EventOutcome string

// Event outcomes.
const (
	OutcomeYes EventOutcome = "yes"
	OutcomeNo  EventOutcome = "no"
)
