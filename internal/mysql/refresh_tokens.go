package mysql

import (
	"database/sql"
	"time"

	"github.com/gavinwade12/sendkey"
	"github.com/google/uuid"
)

type refreshTokenStore struct {
	conn Conn
}

func (s *refreshTokenStore) Create(token sendkey.RefreshToken) error {
	_, err := s.conn.Exec(`
	INSERT INTO refresh_tokens(id, userId, token, createdAtUtc, expiresAtUtc)
	VALUES (?, ?, ?, ?, ?);`,
		mysqlUUID(string(token.ID[:])), mysqlUUID(string(token.UserID[:])), token.Token, token.CreatedAtUTC, token.ExpiresAtUTC)
	return err
}

func (s *refreshTokenStore) FindByTokenAndUser(token string, userID uuid.UUID) (*sendkey.RefreshToken, error) {
	row := s.conn.QueryRow(
		`SELECT id, createdAtUtc, expiresAtUtc FROM refresh_tokens WHERE token = ? AND userId = ?`,
		token, mysqlUUID(userID[:]))
	var (
		id           mysqlUUID
		createdAtUtc time.Time
		expiresAtUtc time.Time
	)

	err := row.Scan(&id, &createdAtUtc, &expiresAtUtc)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	return &sendkey.RefreshToken{
		ID:           id.UUID(),
		UserID:       userID,
		Token:        token,
		CreatedAtUTC: createdAtUtc,
		ExpiresAtUTC: expiresAtUtc,
	}, nil
}

func (s *refreshTokenStore) Delete(id uuid.UUID) error {
	_, err := s.conn.Exec(`DELETE FROM refresh_tokens WHERE id = ?;`, mysqlUUID(id[:]))
	return err
}
