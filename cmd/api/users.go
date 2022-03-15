package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/gavinwade12/sendkey"
	"github.com/gavinwade12/sendkey/internal/app"
	"github.com/google/uuid"
	"github.com/julienschmidt/httprouter"
)

type UsersController struct {
	baseController

	service *app.UserService

	tokenProvider TokenProvider
	refreshTokens RefreshTokenRepository
}

type RefreshTokenRepository interface {
	Create(sendkey.RefreshToken) error
	FindByTokenAndUser(token string, userID uuid.UUID) (*sendkey.RefreshToken, error)
	Delete(uuid.UUID) error
}

func (c *UsersController) CreateUser(w http.ResponseWriter, r *http.Request, _ httprouter.Params) error {
	var req app.CreateUserRequest
	var resp *app.CreateUserResponse

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		resp = &app.CreateUserResponse{Errors: []string{err.Error()}}
		w.WriteHeader(http.StatusBadRequest)
		return json.NewEncoder(w).Encode(resp)
	}

	resp, err := c.service.CreateUser(req)
	if err != nil {
		return err
	}

	if !resp.Success {
		w.WriteHeader(http.StatusBadRequest)
	}
	return json.NewEncoder(w).Encode(resp)
}

func (c *UsersController) Login(w http.ResponseWriter, r *http.Request, _ httprouter.Params) error {
	var req app.UserLoginRequest
	var model struct {
		app.UserLoginResponse
		AccessToken  *Token `json:"accessToken"`
		RefreshToken *Token `json:"refreshToken"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		model.Errors = append(model.Errors, err.Error())
		w.WriteHeader(http.StatusBadRequest)
		return json.NewEncoder(w).Encode(model)
	}

	resp, err := c.service.Login(req)
	if err != nil {
		return err
	}

	model.UserLoginResponse = *resp
	if !resp.Success {
		w.WriteHeader(http.StatusBadRequest)
		return json.NewEncoder(w).Encode(model)
	}

	srt, rt := c.refreshToken(model.User.ID)
	err = c.refreshTokens.Create(srt)
	if err != nil {
		return err
	}
	model.RefreshToken = &rt

	model.AccessToken, err = c.tokenProvider.AccessToken(model.User.ID)
	if err != nil {
		return err
	}

	return json.NewEncoder(w).Encode(model)
}

func (c *UsersController) RefreshToken(w http.ResponseWriter, r *http.Request, p httprouter.Params) error {
	var model struct {
		UserID       uuid.UUID `json:"userId"`
		RefreshToken string    `json:"refreshToken"`
	}

	if err := json.NewDecoder(r.Body).Decode(&model); err != nil {
		return Error{StatusCode: http.StatusBadRequest, Message: err.Error()}
	}

	var response struct {
		Success     bool     `json:"success"`
		Errors      []string `json:"errors"`
		AccessToken *Token   `json:"accessToken"`
	}
	if model.UserID == uuid.Nil {
		response.Errors = append(response.Errors, "Invalid userId.")
	}
	if strings.TrimSpace(model.RefreshToken) == "" {
		response.Errors = append(response.Errors, "A refresh token is required.")
	}
	if len(response.Errors) > 0 {
		w.WriteHeader(http.StatusBadRequest)
		return json.NewEncoder(w).Encode(response)
	}

	rt, err := c.refreshTokens.FindByTokenAndUser(model.RefreshToken, model.UserID)
	if err != nil {
		return err
	}
	if rt == nil {
		response.Errors = append(response.Errors, "Invalid refresh token.")
		w.WriteHeader(http.StatusBadRequest)
		return json.NewEncoder(w).Encode(response)
	}

	response.AccessToken, err = c.tokenProvider.AccessToken(rt.UserID)
	if err != nil {
		return err
	}

	response.Success = true
	return json.NewEncoder(w).Encode(response)
}

func (c *UsersController) refreshToken(userID uuid.UUID) (sendkey.RefreshToken, Token) {
	rt := c.tokenProvider.RefreshToken()

	return sendkey.RefreshToken{
		ID:           uuid.New(),
		UserID:       userID,
		Token:        rt.Token,
		CreatedAtUTC: time.Now().UTC(),
		ExpiresAtUTC: time.Unix(rt.Expires, 0),
	}, rt
}
