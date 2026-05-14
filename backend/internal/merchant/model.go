package merchant

import (
	"time"
)

type Merchant struct {
	ID            string     `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" db:"id"             json:"id"`
	CreatedAt     time.Time  `gorm:"autoCreateTime"                                db:"created_at"     json:"createdAt"`
	UpdatedAt     time.Time  `gorm:"autoUpdateTime"                                db:"updated_at"     json:"updatedAt"`
	BusinessID    string     `gorm:"column:business_id"                            db:"business_id"    json:"businessId"`
	Name          string     `gorm:"column:name"                                   db:"name"           json:"name"`
	Email         string     `gorm:"column:email"                                  db:"email"          json:"email"`
	BankName      string     `gorm:"column:bank_name"                              db:"bank_name"      json:"bankName"`
	AccountNumber string     `gorm:"column:account_number"                         db:"account_number" json:"accountNumber"`
	SuiAddress           string     `gorm:"column:sui_address"             db:"sui_address"            json:"suiAddress"`
	EncryptedPrivateKey  string     `gorm:"column:encrypted_private_key"   db:"encrypted_private_key"  json:"-"`
	Status               string     `gorm:"column:status"                  db:"status"                 json:"status"`
	PasswordHash         string     `gorm:"column:password_hash"           db:"password_hash"          json:"-"`
	DeletedAt     *time.Time `gorm:"index"                                         db:"deleted_at"     json:"-"`
}

type RegisterRequest struct {
	Name          string `json:"name"`
	Email         string `json:"email"`
	Password      string `json:"password"`
	BankName      string `json:"bankName"`
	AccountNumber string `json:"accountNumber"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type AuthResponse struct {
	Merchant *Merchant `json:"merchant"`
	Token    string    `json:"token"`
}

type MerchantStats struct {
	USDCTotalReceived float64 `json:"usdcTotalReceived"`
	USDCToday         float64 `json:"usdcToday"`
	USDCThisWeek      float64 `json:"usdcThisWeek"`
	NGNTotalSettled   float64 `json:"ngnTotalSettled"`
	NGNToday          float64 `json:"ngnToday"`
	NGNThisWeek       float64 `json:"ngnThisWeek"`
	PendingCount      int     `json:"pendingCount"`
	FailedCount       int     `json:"failedCount"`
}

type UpdatePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}
