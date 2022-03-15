package mysql

import (
	"database/sql"
	"time"

	"github.com/gavinwade12/sendkey"
	"github.com/google/uuid"
)

type userStore struct {
	conn Conn
}

const userSelectFrom = `SELECT id, email, emailVerified, firstName, lastName, password, createdAtUtc FROM users`

func (s *userStore) Find(id uuid.UUID) (*sendkey.User, error) {
	row := s.conn.QueryRow(userSelectFrom+` WHERE ID = ?;`, mysqlUUID(id[:]))
	return s.scanUser(row)
}

func (s *userStore) FindByEmail(email string) (*sendkey.User, error) {
	row := s.conn.QueryRow(userSelectFrom+` WHERE Email = ?;`, email)
	return s.scanUser(row)
}

func (s *userStore) Create(u sendkey.User) error {
	_, err := s.conn.Exec(`
	INSERT INTO users(id, email, emailVerified, firstName, lastName, password, createdAtUtc)
	VALUES (?, ?, ?, ?, ?, ?, ?);`,
		mysqlUUID(string(u.ID[:])), u.Email, mysqlBool(u.EmailVerified), u.FirstName, u.LastName, u.Password, u.CreatedAtUTC)
	return err
}

func (s *userStore) Update(u sendkey.User) error {
	_, err := s.conn.Exec(`
	UPDATE users
	SET email = ?, emailVerified = ?, firstName = ?, lastName = ?, password = ?
	WHERE id = ?;`,
		u.Email, u.EmailVerified, u.FirstName, u.LastName, u.Password, mysqlUUID(u.ID[:]))
	return err
}

func (s *userStore) Delete(id uuid.UUID) error {
	_, err := s.conn.Exec(`DELETE FROM users WHERE id = ?;`, mysqlUUID(id[:]))
	return err
}

func (s *userStore) scanUser(row *sql.Row) (*sendkey.User, error) {
	var (
		id            mysqlUUID
		email         string
		emailVerified mysqlBool
		firstName     string
		lastName      string
		password      string
		createdAtUtc  time.Time
	)

	err := row.Scan(&id, &email, &emailVerified, &firstName, &lastName, &password, &createdAtUtc)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	u := &sendkey.User{
		ID:            id.UUID(),
		Email:         email,
		EmailVerified: bool(emailVerified),
		FirstName:     firstName,
		LastName:      lastName,
		Password:      password,
		CreatedAtUTC:  createdAtUtc,
	}

	return u, nil
}
