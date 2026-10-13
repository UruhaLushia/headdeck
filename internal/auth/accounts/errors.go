package accounts

import (
	"errors"

	"modernc.org/sqlite"
)

var ErrConflict = errors.New("account or identity is already linked")

func mutationError(err error) error {
	var dbErr *sqlite.Error
	if errors.As(err, &dbErr) && dbErr.Code()&255 == 19 {
		return ErrConflict
	}
	return err
}
