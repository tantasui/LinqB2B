package model

// Business represents an onboarded B2B client.
// Each business has a unique derivation index used to generate deterministic
// wallet addresses across all supported chains from the platform master mnemonic.
type Business struct {
	BaseModel
	// BusinessID is a human-readable unique identifier supplied at onboarding
	// (e.g. "acme_corp"). Immutable after creation.
	BusinessID string `gorm:"type:varchar(64);not null;uniqueIndex:idx_unique_business_id" json:"businessId"`

	// Name is a display name for the business.
	Name string `gorm:"type:varchar(255);not null" json:"name"`

	// WebhookURL is the endpoint we POST payment events to.
	WebhookURL string `gorm:"type:varchar(2048);not null" json:"webhookUrl"`

	// WebhookSecret is the HMAC-SHA256 signing secret shared with the business.
	// Stored as plain text here — in production this should be encrypted at rest.
	WebhookSecret string `gorm:"type:varchar(512);not null" json:"-"`

	// DerivationIndex is the BIP-44 account index used to derive this business's
	// wallet addresses. Unique and monotonically increasing — never reused.
	DerivationIndex uint32 `gorm:"not null;uniqueIndex:idx_unique_derivation_index" json:"derivationIndex"`

	// Active controls whether events are dispatched to this business.
	Active bool `gorm:"not null;default:true" json:"active"`
}