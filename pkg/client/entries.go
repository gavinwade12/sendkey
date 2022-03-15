package client

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gavinwade12/sendkey"
	"github.com/google/uuid"
)

type entriesResource struct {
	c *Client
}

type CreateEntryRequest struct {
	Name            string    `json:"name"`
	SenderID        uuid.UUID `json:"senderId"`
	SendToEmail     string    `json:"sendToEmail"`
	Value           string    `json:"value"`
	Secret          string    `json:"secret"`
	DurationMinutes int       `json:"duration"`
}

type CreateEntryResponse struct {
	Success bool           `json:"success"`
	Errors  []string       `json:"errors"`
	Entry   *sendkey.Entry `json:"entry"`
}

func (r *entriesResource) CreateEntry(model CreateEntryRequest) (*CreateEntryResponse, *Error, error) {
	const path = `/entries`

	jr, err := jsonReader(model)
	if err != nil {
		return nil, nil, err
	}

	res, err := r.c.doRequest(http.MethodPost, path, jr)
	if err != nil {
		return nil, nil, err
	}
	if res.StatusCode > http.StatusBadRequest {
		e, err := r.c.parseErrorResponse(res)
		return nil, e, err
	}
	defer res.Body.Close()

	var response CreateEntryResponse
	err = json.NewDecoder(res.Body).Decode(&response)
	if err != nil {
		return nil, nil, err
	}

	return &response, nil, nil
}

func (r *entriesResource) ListEntries() ([]sendkey.Entry, *Error, error) {
	path := fmt.Sprintf("/users/%s/entries", r.c.currentUserID.String())

	res, err := r.c.doRequest(http.MethodGet, path, nil)
	if err != nil {
		return nil, nil, err
	}
	if res.StatusCode > http.StatusBadRequest {
		e, err := r.c.parseErrorResponse(res)
		return nil, e, err
	}
	defer res.Body.Close()

	var response []sendkey.Entry
	err = json.NewDecoder(res.Body).Decode(&response)
	if err != nil {
		return nil, nil, fmt.Errorf("decoding response: %w", err)
	}

	return response, nil, nil
}
