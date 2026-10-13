package web

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/UruhaLushia/headdeck/internal/auth/principal"
	"github.com/UruhaLushia/headdeck/internal/database"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type session struct {
	Subject   string
	Role      principal.Role
	ExpiresAt time.Time
	CreatedAt time.Time
	AvatarURL string
}

type sessionRecord = database.SessionRecord

type sessionStore struct {
	db       *sql.DB
	orm      *gorm.DB
	key      []byte
	keyID    string
	ttl      time.Duration
	capacity int
}

func newSessionStore(db *sql.DB, key []byte, ttl time.Duration) (*sessionStore, error) {
	if db == nil {
		return nil, errors.New("web sessions require a settings database")
	}
	if ttl <= 0 {
		ttl = time.Hour
	}
	digest := sha256.Sum256(key)
	orm, err := database.Open(db)
	if err != nil {
		return nil, err
	}
	s := &sessionStore{db: db, orm: orm, key: key, keyID: hex.EncodeToString(digest[:]), ttl: ttl, capacity: 4096}
	if !orm.Migrator().HasTable(&sessionRecord{}) {
		if err = orm.Migrator().CreateTable(&sessionRecord{}); err != nil {
			return nil, err
		}
	}
	err = orm.Where(clause.Or(clause.Lte{Column: "expires", Value: time.Now().Unix()}, clause.Neq{Column: "key_id", Value: s.keyID})).Delete(&sessionRecord{}).Error
	return s, err
}

func sessionHash(value string) string {
	hash := sha256.Sum256([]byte(value))
	return hex.EncodeToString(hash[:])
}

func sessionOwner(subject string) (string, error) {
	if encoded, ok := strings.CutPrefix(subject, "account:"); ok {
		payload, err := base64.RawURLEncoding.DecodeString(encoded)
		if err != nil {
			return "", err
		}
		var account accountSession
		if err := json.Unmarshal(payload, &account); err != nil {
			return "", err
		}
		if account.ID == "" {
			return "", errors.New("session account is missing")
		}
		return "account:" + account.ID, nil
	}
	return "token:" + subject, nil
}

func (s *sessionStore) issue(subject string, role principal.Role, now time.Time, agent string) (string, error) {
	owner, err := sessionOwner(subject)
	if err != nil {
		return "", err
	}
	payload := fmt.Sprintf("%s|%s|%d|%s|%s", subject, role, now.Add(s.ttl).Unix(), s.keyID, rand.Text())
	mac := hmac.New(sha256.New, s.key)
	mac.Write([]byte(payload))
	value := base64.RawURLEncoding.EncodeToString([]byte(payload)) + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if len(value) > 3800 {
		return "", errors.New("identity is too large for a session cookie")
	}
	err = s.orm.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where(clause.Lte{Column: "expires", Value: now.Unix()}).Delete(&sessionRecord{}).Error; err != nil {
			return err
		}
		var count int64
		if err := tx.Model(&sessionRecord{}).Count(&count).Error; err != nil {
			return err
		}
		if count >= int64(s.capacity) {
			return errors.New("too many active sessions")
		}
		if err := tx.Model(&sessionRecord{}).Where(&sessionRecord{Owner: owner}).Count(&count).Error; err != nil {
			return err
		}
		if count >= 32 {
			return errors.New("too many active sessions for this account; sign out on another device")
		}
		return tx.Create(&sessionRecord{Hash: sessionHash(value), Owner: owner, Created: now.Unix(), Expires: now.Add(s.ttl).Unix(), KeyID: s.keyID, Agent: truncateAgent(agent)}).Error
	})
	return value, err
}

func truncateAgent(value string) string {
	if len(value) > 256 {
		return value[:256]
	}
	return value
}

func (s *sessionStore) verify(value string, now time.Time) (session, bool) {
	if len(value) > 3800 {
		return session{}, false
	}
	payloadPart, sigPart, ok := strings.Cut(value, ".")
	if !ok {
		return session{}, false
	}
	payload, err := base64.RawURLEncoding.DecodeString(payloadPart)
	if err != nil {
		return session{}, false
	}
	signature, err := base64.RawURLEncoding.DecodeString(sigPart)
	if err != nil {
		return session{}, false
	}
	mac := hmac.New(sha256.New, s.key)
	mac.Write(payload)
	if !hmac.Equal(mac.Sum(nil), signature) {
		return session{}, false
	}
	fields := strings.Split(string(payload), "|")
	if len(fields) != 5 || fields[4] == "" || fields[3] != s.keyID {
		return session{}, false
	}
	// The database owns expiry so renewal can keep the cookie and device ID stable.
	var record sessionRecord
	if err := s.orm.Where(&sessionRecord{Hash: sessionHash(value), KeyID: s.keyID}).First(&record).Error; err != nil || now.Unix() >= record.Expires {
		return session{}, false
	}
	return session{Subject: fields[0], Role: principal.Role(fields[1]), ExpiresAt: time.Unix(record.Expires, 0), CreatedAt: time.Unix(record.Created, 0)}, true
}

func (s *sessionStore) renew(value string, current session, now time.Time) (bool, error) {
	result := s.orm.Model(&sessionRecord{}).
		Where(&sessionRecord{Hash: sessionHash(value), KeyID: s.keyID, Expires: current.ExpiresAt.Unix()}).
		Where(clause.Gt{Column: "expires", Value: now.Unix()}).
		Update("expires", now.Add(s.ttl).Unix())
	return result.RowsAffected == 1, result.Error
}

func (s *sessionStore) revoke(value string) error {
	return s.orm.Where(&sessionRecord{Hash: sessionHash(value)}).Delete(&sessionRecord{}).Error
}
