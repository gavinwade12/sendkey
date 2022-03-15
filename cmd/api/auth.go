package main

import (
	"encoding/hex"
	"fmt"
	"math/rand"
	"net/http"
	"time"

	jwt "github.com/dgrijalva/jwt-go"
	"github.com/google/uuid"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

// Token is a token, used for authentication, with a Unix time expiration date
type Token struct {
	Token   string `json:"token"`
	Expires int64  `json:"expires"`
}

// TokenProvider defines the methods necessary for providing access tokens
type TokenProvider interface {
	AccessToken(userID uuid.UUID) (*Token, error)
	RefreshToken() Token
}

// AccessTokenVerifier defines the methods necessary for verifying auth tokens
type AccessTokenVerifier interface {
	Verify(string) (uuid.UUID, error) // Verify should return the UserID from the token if it's valid, otherwise it should return an error
}

type tokenManager struct {
	privateKey           []byte
	accessTokenLifetime  time.Duration
	refreshTokenLifetime time.Duration
}

var _ TokenProvider = (*tokenManager)(nil)
var _ AccessTokenVerifier = (*tokenManager)(nil)

func newAuthTokenManager(privateKey []byte, accessTokenLifetime, refreshTokenLifetime time.Duration) *tokenManager {
	return &tokenManager{privateKey, accessTokenLifetime, refreshTokenLifetime}
}

func (m *tokenManager) AccessToken(userID uuid.UUID) (*Token, error) {
	now := time.Now()
	expires := now.Add(m.accessTokenLifetime).Unix()
	claims := &jwt.StandardClaims{
		ExpiresAt: expires,
		Id:        userID.String(),
		IssuedAt:  now.Unix(),
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.privateKey)
	if err != nil {
		return nil, err
	}

	return &Token{
		Token:   token,
		Expires: expires,
	}, nil
}

func (m *tokenManager) RefreshToken() Token {
	b := make([]byte, 25)
	rand.Read(b)

	return Token{
		Token:   hex.EncodeToString(b),
		Expires: time.Now().UTC().Add(m.refreshTokenLifetime).Unix(),
	}
}

func (m *tokenManager) Verify(token string) (uuid.UUID, error) {
	if token == "" {
		return uuid.Nil, Error{StatusCode: http.StatusUnauthorized, Message: "no token provided"}
	}

	t, err := jwt.Parse(token, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, Error{StatusCode: http.StatusUnauthorized, Message: fmt.Sprintf("unexpected signing method: %v", token.Header["alg"])}
		}
		return m.privateKey, nil
	})
	if err != nil {
		if _, ok := err.(*jwt.ValidationError); ok {
			return uuid.Nil, Error{StatusCode: http.StatusUnauthorized, Message: err.Error()}
		}

		return uuid.Nil, err
	}

	claims, ok := t.Claims.(jwt.MapClaims)
	if !ok || !t.Valid {
		return uuid.Nil, Error{StatusCode: http.StatusUnauthorized, Message: "token invalid or failed to parse token claims"}
	}

	idClaim, ok := claims["jti"].(string)
	if !ok {
		return uuid.Nil, Error{StatusCode: http.StatusUnauthorized, Message: "invalid token claims"}
	}

	id, err := uuid.Parse(idClaim)
	if err != nil {
		return uuid.Nil, Error{StatusCode: http.StatusUnauthorized, Message: "invalid token claims"}
	}

	return id, nil
}
