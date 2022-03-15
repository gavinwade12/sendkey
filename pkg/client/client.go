package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
)

var DefaultHTTPClient = &http.Client{
	Timeout: time.Second * 10,
}

type Client struct {
	baseURL        string
	client         *http.Client
	defaultHeaders map[string][]string

	accessToken   string
	refreshToken  string
	currentUserID uuid.UUID

	Users   *usersResource
	Entries *entriesResource
}

type Option func(c *Client)

var WithHTTPClient = func(client *http.Client) Option {
	return func(c *Client) {
		c.client = client
	}
}

var WithDefaultHeaders = func(headers map[string][]string) Option {
	return func(c *Client) {
		c.defaultHeaders = headers
	}
}

var WithSession = func(userID uuid.UUID, refreshToken, accessToken string) Option {
	return func(c *Client) {
		c.currentUserID = userID
		c.refreshToken = refreshToken
		c.accessToken = accessToken
	}
}

func NewClient(baseURL string, opts ...Option) *Client {
	client := &Client{
		baseURL: baseURL,
	}
	for _, opt := range opts {
		opt(client)
	}

	if client.client == nil {
		client.client = DefaultHTTPClient
	}

	client.Users = &usersResource{client}
	client.Entries = &entriesResource{client}

	return client
}

func (c *Client) doRequest(method, path string, body io.ReadSeeker) (*http.Response, error) {
	req, err := http.NewRequest(method, c.baseURL+path, body)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Accept", "application/json")

	for key, values := range c.defaultHeaders {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}

	if c.accessToken != "" && path != "/token" && path != "/login" {
		req.Header.Set("Authorization", "Bearer "+c.accessToken)
	}

	res, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	if res.StatusCode != http.StatusUnauthorized || c.refreshToken == "" ||
		path == "/token" || path == "/login" {
		return res, nil
	}

	e, err := c.refreshAccessToken()
	if err != nil {
		return nil, err
	}
	if e != nil {
		return nil, fmt.Errorf("fetching access token: [%d]: %s", e.StatusCode, e.Message)
	}

	return c.client.Do(req)
}

func (c *Client) refreshAccessToken() (*Error, error) {
	const path = `/token`

	jr, err := jsonReader(map[string]string{
		"userId":       c.currentUserID.String(),
		"refreshToken": c.refreshToken,
	})
	if err != nil {
		return nil, err
	}

	res, err := c.doRequest(http.MethodPost, path, jr)
	if err != nil {
		return nil, err
	}
	if res.StatusCode != http.StatusOK {
		return c.parseErrorResponse(res)
	}
	defer res.Body.Close()

	var token Token
	err = json.NewDecoder(res.Body).Decode(&token)
	if err != nil {
		return nil, err
	}

	c.accessToken = token.Token
	return nil, nil
}

func jsonReader(value interface{}) (io.ReadSeeker, error) {
	b, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}

	return bytes.NewReader(b), nil
}

type Error struct {
	UserID     uuid.UUID `json:"userId"`
	StatusCode int       `json:"statusCode"`
	Message    string    `json:"message"`
}

func (c *Client) parseErrorResponse(res *http.Response) (*Error, error) {
	defer res.Body.Close()

	var e Error
	err := json.NewDecoder(res.Body).Decode(&e)
	if err != nil {
		return nil, fmt.Errorf("decoding error response [status: %d]: %w ", res.StatusCode, err)
	}

	return &e, nil
}

type Token struct {
	Token   string `json:"token"`
	Expires int64  `json:"expires"`
}
