package main

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gavinwade12/sendkey/internal/app"
	"github.com/google/uuid"
	"github.com/julienschmidt/httprouter"
)

type EntriesController struct {
	baseController

	service *app.EntryService
}

func (s *EntriesController) CreateEntry(w http.ResponseWriter, r *http.Request, _ httprouter.Params) error {
	userID, err := s.GetCurrentUserID(r)
	if err != nil {
		return Error{StatusCode: http.StatusUnauthorized, Message: err.Error()}
	}
	if userID == uuid.Nil {
		return Error{UserID: userID, StatusCode: http.StatusUnauthorized}
	}

	var req app.CreateEntryRequest
	var resp *app.CreateEntryResponse
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		resp = &app.CreateEntryResponse{Errors: []string{err.Error()}}
		w.WriteHeader(http.StatusBadRequest)
		return json.NewEncoder(w).Encode(resp)
	}
	req.SenderID = userID
	req.Duration = req.Duration * time.Minute

	resp, err = s.service.CreateEntry(req)
	if err != nil {
		return err
	}

	if !resp.Success {
		w.WriteHeader(http.StatusBadRequest)
	}
	return json.NewEncoder(w).Encode(resp)
}

func (c *EntriesController) FindEntry(w http.ResponseWriter, r *http.Request, p httprouter.Params) error {
	userID, err := c.GetCurrentUserID(r)
	if err != nil {
		return Error{StatusCode: http.StatusUnauthorized, Message: err.Error()}
	}

	entryID, err := uuid.Parse(p.ByName("entryID"))
	if err != nil {
		return err
	}

	nonce := r.URL.Query().Get("nonce")
	if nonce == "" {
		return Error{UserID: userID, StatusCode: http.StatusBadRequest, Message: "A nonce is required."}
	}

	entry, err := c.service.FindEntry(entryID, nonce)
	if err != nil {
		return err
	}
	if entry == nil {
		return Error{UserID: userID, StatusCode: http.StatusNotFound}
	}

	return json.NewEncoder(w).Encode(entry)
}

func (c *EntriesController) FindUserEntries(w http.ResponseWriter, r *http.Request, p httprouter.Params) error {
	currentUserID, err := c.GetCurrentUserID(r)
	if err != nil {
		return Error{StatusCode: http.StatusUnauthorized, Message: err.Error()}
	}

	userID, err := uuid.Parse(p.ByName("userID"))
	if err != nil {
		return Error{UserID: currentUserID, StatusCode: http.StatusBadRequest, Message: "Invalid userID."}
	}
	if currentUserID.String() != userID.String() {
		return Error{UserID: currentUserID, StatusCode: http.StatusForbidden}
	}

	entries, err := c.service.FindByUserID(userID)
	if err != nil {
		return err
	}

	return json.NewEncoder(w).Encode(entries)
}

func (c *EntriesController) EntryValue(w http.ResponseWriter, r *http.Request, p httprouter.Params) error {
	userID, err := c.GetCurrentUserID(r)
	if err != nil {
		return err
	}

	entryID, err := uuid.Parse(p.ByName("entryID"))
	if err != nil {
		return err
	}

	nonce := r.URL.Query().Get("nonce")
	if nonce == "" {
		return Error{UserID: userID, StatusCode: http.StatusBadRequest, Message: "A nonce is required."}
	}
	secret := r.URL.Query().Get("secret")
	if secret == "" {
		return Error{UserID: userID, StatusCode: http.StatusBadRequest, Message: "A secret is required."}
	}

	resp, err := c.service.DecryptEntry(app.DecryptEntryRequest{
		ID:     entryID,
		Nonce:  nonce,
		Secret: secret,
	})
	if err != nil {
		return err
	}

	type response struct {
		Success bool     `json:"success"`
		Errors  []string `json:"errors"`
		Value   *string  `json:"value"`
	}
	model := response{
		Success: resp.Success,
		Errors:  resp.Errors,
	}
	if resp.Entry != nil {
		v := string(resp.Entry.Value)
		model.Value = &v
	}

	return json.NewEncoder(w).Encode(model)
}
