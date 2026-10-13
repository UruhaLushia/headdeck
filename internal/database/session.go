package database

type SessionRecord struct {
	Hash    string `gorm:"primaryKey"`
	Owner   string `gorm:"not null;index:web_sessions_owner"`
	Created int64  `gorm:"not null"`
	Expires int64  `gorm:"not null;index:web_sessions_expiry"`
	KeyID   string `gorm:"not null"`
	Agent   string `gorm:"not null"`
}

func (SessionRecord) TableName() string { return "web_sessions" }
