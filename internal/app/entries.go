package app

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/gavinwade12/sendkey"
	"github.com/google/uuid"
)

type EntryRepository interface {
	Find(uuid.UUID) (*sendkey.Entry, error)
	FindByUserID(uuid.UUID) ([]sendkey.Entry, error)
	Create(sendkey.Entry) error
	Delete(uuid.UUID) error
	IncrementInvalidAttempts(uuid.UUID) (int, error)

	CreateClaimedEntry(sendkey.ClaimedEntry) error
	CreateExpiredEntry(sendkey.ExpiredEntry) error
}

type EntryService struct {
	entries EntryRepository

	aesKey      []byte
	maxAttempts int
}

// The key argument should be the AES key, either 16, 24, or 32 bytes to select AES-128, AES-192, or AES-256.
// The maxAttempts argument is the number of invalid attempts allowed before an entry is forcefully expired.
func NewEntryService(er EntryRepository, key []byte, maxAttempts int) *EntryService {
	return &EntryService{er, key, maxAttempts}
}

type CreateEntryRequest struct {
	Name        string        `json:"name"`
	SenderID    uuid.UUID     `json:"senderId"`
	SendToEmail string        `json:"sendToEmail"`
	Value       string        `json:"value"`
	Secret      string        `json:"secret"`
	Duration    time.Duration `json:"duration"`
}

type CreateEntryResponse struct {
	Success bool           `json:"success"`
	Errors  []string       `json:"errors"`
	Entry   *sendkey.Entry `json:"entry"`
}

func (s *EntryService) CreateEntry(req CreateEntryRequest) (*CreateEntryResponse, error) {
	resp := &CreateEntryResponse{}
	if req.SenderID == uuid.Nil {
		resp.Errors = append(resp.Errors, "A sender ID is required.")
	}
	if strings.TrimSpace(req.Name) == "" {
		resp.Errors = append(resp.Errors, "A name is required.")
	}
	req.SendToEmail = strings.TrimSpace(req.SendToEmail)
	if req.SendToEmail == "" {
		resp.Errors = append(resp.Errors, "A send to email is required.")
	}
	if strings.TrimSpace(req.Value) == "" {
		resp.Errors = append(resp.Errors, "A value is required.")
	}
	if strings.TrimSpace(req.Secret) == "" {
		resp.Errors = append(resp.Errors, "A secret is required.")
	}
	if req.Duration <= 0 {
		resp.Errors = append(resp.Errors, "Duration must be greater than 0.")
	}
	if len(resp.Errors) > 0 {
		resp.Success = false
		return resp, nil
	}

	nonce := s.nonce()
	value, err := s.encrypt([]byte(req.Value), nonce, []byte(req.Secret))
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	entry := sendkey.Entry{
		ID:           uuid.New(),
		Name:         req.Name,
		SentByUserID: req.SenderID,
		SentToEmail:  req.SendToEmail,
		Nonce:        nonce,
		Value:        value,
		CreatedAtUTC: now,
		ExpiresAtUTC: now.Add(req.Duration),
	}

	err = s.entries.Create(entry)
	if err != nil {
		return nil, err
	}
	// TODO: remove
	fmt.Println(hex.EncodeToString(entry.Nonce))

	err = s.SendEntry(entry)
	if err != nil {
		// TODO: delete entry? attempt to resend?
		return nil, err
	}

	resp.Success = true
	resp.Entry = &entry
	return resp, nil
}

func (s *EntryService) SendEntry(entry sendkey.Entry) error {
	// TODO: add email client to service and send email
	return nil
}

func (s *EntryService) FindEntry(id uuid.UUID, nonce string) (*sendkey.Entry, error) {
	entry, err := s.entries.Find(id)
	if err != nil || entry == nil {
		return entry, err
	}
	if !entry.ExpiresAtUTC.After(time.Now().UTC()) {
		_, err = s.expireEntry(*entry, false)
		return nil, err
	}

	if hex.EncodeToString(entry.Nonce) != nonce {
		return nil, nil
	}

	return entry, nil
}

func (s *EntryService) FindByUserID(userID uuid.UUID) ([]sendkey.Entry, error) {
	entries, err := s.entries.FindByUserID(userID)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	result := []sendkey.Entry{}
	for _, entry := range entries {
		if entry.ExpiresAtUTC.After(now) {
			result = append(result, entry)
			continue
		}

		if _, err = s.expireEntry(entry, false); err != nil {
			return nil, err
		}
	}

	return result, nil
}

type DecryptEntryRequest struct {
	ID     uuid.UUID `json:"id"`
	Nonce  string    `json:"nonce"`
	Secret string    `json:"secret"`
}

type DecryptEntryResponse struct {
	Success bool           `json:"success"`
	Errors  []string       `json:"errors"`
	Expired bool           `json:"expired"`
	Entry   *sendkey.Entry `json:"entry"`
}

func (s *EntryService) DecryptEntry(req DecryptEntryRequest) (*DecryptEntryResponse, error) {
	resp := &DecryptEntryResponse{}

	entry, err := s.FindEntry(req.ID, req.Nonce)
	if err != nil {
		return nil, err
	}
	if entry == nil {
		resp.Errors = append(resp.Errors, "Invalid entry ID.")
		return resp, nil
	}

	value, err := s.decrypt(entry.Value, entry.Nonce, []byte(req.Secret))
	if err != nil {
		resp.Errors = append(resp.Errors, "Invalid secret.")

		ee, err := s.incrementInvalidAttempts(*entry)
		if err != nil {
			return nil, err
		}

		if ee != nil {
			resp.Expired = true
			resp.Errors = append(resp.Errors, "Too many attempts have been made, and the entry has been expired.")
		}

		return resp, nil
	}

	_, err = s.claimEntry(*entry)
	if err != nil {
		return nil, err
	}

	entry.Value = value
	resp.Entry = entry
	resp.Success = true
	return resp, nil
}

func (s *EntryService) encrypt(value, nonce, secret []byte) ([]byte, error) {
	key := sha256.Sum256(append(s.aesKey, secret...))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}

	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	return aead.Seal(nil, nonce, value, nil), nil
}

func (s *EntryService) decrypt(value, nonce, secret []byte) ([]byte, error) {
	key := sha256.Sum256(append(s.aesKey, secret...))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}

	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	return aead.Open(nil, nonce, value, nil)
}

func (s *EntryService) nonce() []byte {
	b := make([]byte, 12)
	rand.Read(b)
	return b
}

func (s *EntryService) expireEntry(e sendkey.Entry, tooManyAttempts bool) (*sendkey.ExpiredEntry, error) {
	ee := sendkey.ExpiredEntry{
		EntryID:         e.ID,
		Name:            e.Name,
		SentByUserID:    e.SentByUserID,
		SentToEmail:     e.SentToEmail,
		TooManyAttempts: tooManyAttempts,
		ExpiredAtUTC:    time.Now().UTC(),
	}
	err := s.entries.CreateExpiredEntry(ee)
	if err != nil {
		return nil, err
	}

	err = s.entries.Delete(e.ID)
	if err != nil {
		return nil, err
	}

	return &ee, nil
}

func (s *EntryService) incrementInvalidAttempts(e sendkey.Entry) (*sendkey.ExpiredEntry, error) {
	attempts, err := s.entries.IncrementInvalidAttempts(e.ID)
	if err != nil {
		return nil, err
	}

	if attempts >= s.maxAttempts {
		return s.expireEntry(e, true)
	}

	return nil, nil
}

func (s *EntryService) claimEntry(e sendkey.Entry) (*sendkey.ClaimedEntry, error) {
	ce := sendkey.ClaimedEntry{
		EntryID:      e.ID,
		Name:         e.Name,
		SentByUserID: e.SentByUserID,
		SentToEmail:  e.SentToEmail,
		ClaimedAtUTC: time.Now().UTC(),
	}
	err := s.entries.CreateClaimedEntry(ce)
	if err != nil {
		return nil, err
	}

	err = s.entries.Delete(e.ID)
	if err != nil {
		return nil, err
	}

	return &ce, nil
}
