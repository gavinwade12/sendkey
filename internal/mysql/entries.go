package mysql

import (
	"database/sql"
	"time"

	"github.com/gavinwade12/sendkey"
	"github.com/google/uuid"
)

type entryStore struct {
	conn Conn
}

func (s *entryStore) Create(e sendkey.Entry) error {
	_, err := s.conn.Exec(`
	INSERT INTO entries(id, name, sentByUserId, sentToEmail, nonce, value, invalidAttempts, createdAtUtc, expiresAtUtc)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?);`,
		mysqlUUID(e.ID[:]), e.Name, mysqlUUID(e.SentByUserID[:]), e.SentToEmail,
		string(e.Nonce), string(e.Value), e.InvalidAttempts, e.CreatedAtUTC, e.ExpiresAtUTC)
	return err
}

func (s *entryStore) Find(id uuid.UUID) (*sendkey.Entry, error) {
	row := s.conn.QueryRow(
		`SELECT name, sentByUserId, sentToEmail, nonce, value, invalidAttempts, createdAtUtc, expiresAtUtc FROM entries WHERE id = ?;`,
		mysqlUUID(string(id[:])))
	var (
		name            string
		sentByUserId    mysqlUUID
		sentToEmail     string
		nonce           string
		value           string
		invalidAttempts int
		createdAtUtc    time.Time
		expiresAtUtc    time.Time
	)

	err := row.Scan(&name, &sentByUserId, &sentToEmail, &nonce, &value, &invalidAttempts, &createdAtUtc, &expiresAtUtc)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	return &sendkey.Entry{
		ID:              id,
		Name:            name,
		SentByUserID:    sentByUserId.UUID(),
		SentToEmail:     sentToEmail,
		Nonce:           []byte(nonce),
		Value:           []byte(value),
		InvalidAttempts: invalidAttempts,
		CreatedAtUTC:    createdAtUtc,
		ExpiresAtUTC:    expiresAtUtc,
	}, nil
}

func (s *entryStore) FindByUserID(userID uuid.UUID) ([]sendkey.Entry, error) {
	rows, err := s.conn.Query(`
SELECT id, name, sentToEmail, nonce, value, invalidAttempts, createdAtUtc, expiresAtUtc
FROM entries
WHERE sentByUserId = ?
ORDER BY createdAtUtc;`,
		mysqlUUID(userID[:]),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var (
		id              mysqlUUID
		name            string
		sentToEmail     string
		nonce           string
		value           string
		invalidAttempts int
		createdAtUtc    time.Time
		expiresAtUtc    time.Time

		result = []sendkey.Entry{}
	)
	for rows.Next() {
		err = rows.Scan(&id, &name, &sentToEmail, &nonce, &value, &invalidAttempts, &createdAtUtc, &expiresAtUtc)
		if err != nil {
			return nil, err
		}

		result = append(result, sendkey.Entry{
			ID:              id.UUID(),
			Name:            name,
			SentByUserID:    userID,
			SentToEmail:     sentToEmail,
			Nonce:           []byte(nonce),
			Value:           []byte(value),
			InvalidAttempts: invalidAttempts,
			CreatedAtUTC:    createdAtUtc,
			ExpiresAtUTC:    expiresAtUtc,
		})
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}

	return result, nil
}

func (s *entryStore) Delete(id uuid.UUID) error {
	_, err := s.conn.Exec(`DELETE FROM entries WHERE id = ?;`, mysqlUUID(id[:]))
	return err
}

func (s *entryStore) IncrementInvalidAttempts(id uuid.UUID) (int, error) {
	_, err := s.conn.Exec(`UPDATE entries SET invalidAttempts = invalidAttempts + 1 WHERE id = ?;`, mysqlUUID(id[:]))
	if err != nil {
		return 0, err
	}

	row := s.conn.QueryRow(`SELECT invalidAttempts FROM entries WHERE id = ?;`, mysqlUUID(id[:]))
	var attempts int
	err = row.Scan(&attempts)

	return attempts, err
}

func (s *entryStore) CreateClaimedEntry(ce sendkey.ClaimedEntry) error {
	_, err := s.conn.Exec(`
	INSERT INTO claimed_entries(entryId, name, sentByUserId, sentToEmail, claimedAtUtc)
	VALUES (?, ?, ?, ?, ?);`,
		mysqlUUID(ce.EntryID[:]), ce.Name, mysqlUUID(ce.SentByUserID[:]), ce.SentToEmail,
		ce.ClaimedAtUTC)
	return err
}

func (s *entryStore) CreateExpiredEntry(ee sendkey.ExpiredEntry) error {
	_, err := s.conn.Exec(`
	INSERT INTO expired_entries(entryId, name, sentByUserId, sentToEmail, tooManyAttempts, expiredAtUtc)
	VALUES (?, ?, ?, ?, ?, ?);`,
		mysqlUUID(ee.EntryID[:]), ee.Name, mysqlUUID(ee.SentByUserID[:]), ee.SentToEmail,
		ee.TooManyAttempts, ee.ExpiredAtUTC)
	return err
}
