package client

import (
	"encoding/json"
	"net/http"

	"github.com/gavinwade12/sendkey"
)

type usersResource struct {
	c *Client
}

type CreateUserRequest struct {
	Email     string `json:"email"`
	Password  string `json:"password"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
}

type CreateUserResponse struct {
	Success bool          `json:"success"`
	Errors  []string      `json:"errors"`
	User    *sendkey.User `json:"user"`
}

func (r *usersResource) CreateUser(model CreateUserRequest) (*CreateUserResponse, *Error, error) {
	const path = `/users`

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

	var response CreateUserResponse
	err = json.NewDecoder(res.Body).Decode(&response)
	if err != nil {
		return nil, nil, err
	}

	return &response, nil, nil
}

type LoginResponseModel struct {
	Success      bool          `json:"success"`
	Errors       []string      `json:"errors"`
	User         *sendkey.User `json:"user"`
	AccessToken  *Token        `json:"accessToken"`
	RefreshToken *Token        `json:"refreshToken"`
}

func (r *usersResource) Login(email, password string) (*LoginResponseModel, *Error, error) {
	const path = `/login`

	jr, err := jsonReader(map[string]string{
		"email":    email,
		"password": password,
	})
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

	var response LoginResponseModel
	err = json.NewDecoder(res.Body).Decode(&response)
	if err != nil {
		return nil, nil, err
	}

	if response.Success {
		r.c.refreshToken = response.RefreshToken.Token
		r.c.accessToken = response.AccessToken.Token
		r.c.currentUserID = response.User.ID
	}

	return &response, nil, nil
}
