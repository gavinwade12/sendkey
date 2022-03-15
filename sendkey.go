package sendkey

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID            uuid.UUID `json:"id"`
	Email         string    `json:"email"`
	EmailVerified bool      `json:"emailVerified"`
	FirstName     string    `json:"firstName"`
	LastName      string    `json:"lastName"`
	Password      string    `json:"-"`
	CreatedAtUTC  time.Time `json:"createdAtUtc"`
}

type Entry struct {
	ID              uuid.UUID `json:"id"`
	Name            string    `json:"name"`
	SentByUserID    uuid.UUID `json:"sentByUserId"`
	SentToEmail     string    `json:"sentToEmail"`
	Nonce           []byte    `json:"-"`
	Value           []byte    `json:"-"`
	InvalidAttempts int       `json:"invalidAttempts"`
	CreatedAtUTC    time.Time `json:"createdAtUtc"`
	ExpiresAtUTC    time.Time `json:"expiresAtUtc"`
}

type ClaimedEntry struct {
	EntryID      uuid.UUID `json:"entryId"`
	Name         string    `json:"name"`
	SentByUserID uuid.UUID `json:"sentByUserId"`
	SentToEmail  string    `json:"sentToEmail"`
	ClaimedAtUTC time.Time `json:"claimedAtUtc"`
}

type ExpiredEntry struct {
	EntryID         uuid.UUID `json:"entryId"`
	Name            string    `json:"name"`
	SentByUserID    uuid.UUID `json:"sentByUserId"`
	SentToEmail     string    `json:"sentToEmail"`
	TooManyAttempts bool      `json:"tooManyAttempts"`
	ExpiredAtUTC    time.Time `json:"expiredAtUtc"`
}

type RefreshToken struct {
	ID           uuid.UUID `json:"id"`
	UserID       uuid.UUID `json:"userId"`
	Token        string    `json:"token"`
	CreatedAtUTC time.Time `json:"createdAtUtc"`
	ExpiresAtUTC time.Time `json:"expiresAtUtc"`
}
