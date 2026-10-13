package accounts

import (
	"context"
	"errors"
	"net/url"
	"strings"

	"gorm.io/gorm/clause"
)

func (s *Store) SetLabel(ctx context.Context, id, label string) error {
	label = strings.TrimSpace(label)
	if label == "" || len(label) > 256 {
		return errors.New("account label must contain 1 to 256 bytes")
	}
	result := s.db.WithContext(ctx).Model(&accountRecord{}).Where(&accountRecord{ID: id}).Update("label", label)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) SetAvatar(ctx context.Context, id, value string) error {
	u, err := url.Parse(value)
	if err != nil || len(value) > 512 || u.Scheme != "https" || u.Hostname() == "" || u.User != nil {
		return nil
	}
	return s.db.WithContext(ctx).Clauses(clause.OnConflict{UpdateAll: true}).Create(&profileRecord{AccountID: id, AvatarURL: value}).Error
}

func githubAvatarURL(subject string) string {
	if subject == "" || len(subject) > 20 || strings.Trim(subject, "0123456789") != "" {
		return ""
	}
	return "https://avatars.githubusercontent.com/u/" + subject
}
