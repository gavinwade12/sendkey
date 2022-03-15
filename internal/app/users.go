package app

import (
	"strings"
	"time"

	"github.com/gavinwade12/sendkey"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type UserRepository interface {
	Find(uuid.UUID) (*sendkey.User, error)
	FindByEmail(string) (*sendkey.User, error)
	Create(sendkey.User) error
	Update(sendkey.User) error
	Delete(uuid.UUID) error
}

type UserService struct {
	users UserRepository
}

func NewUserService(users UserRepository) *UserService {
	return &UserService{users}
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

func (s *UserService) CreateUser(req CreateUserRequest) (*CreateUserResponse, error) {
	resp := &CreateUserResponse{}

	req.Email = strings.TrimSpace(req.Email)
	if req.Email == "" {
		resp.Errors = append(resp.Errors, "An email is required.")
	}
	if req.Password == "" {
		resp.Errors = append(resp.Errors, "A password is required.")
	}
	if len(resp.Errors) > 0 {
		resp.Success = false
		return resp, nil
	}

	u, err := s.users.FindByEmail(req.Email)
	if err != nil {
		return nil, err
	}
	if u != nil {
		resp.Errors = append(resp.Errors, "An account with the specified email already exists.")
		resp.Success = false
		return resp, nil
	}

	pass, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	user := sendkey.User{
		ID:           uuid.New(),
		Email:        req.Email,
		FirstName:    req.FirstName,
		LastName:     req.LastName,
		Password:     string(pass),
		CreatedAtUTC: time.Now().UTC(),
	}
	err = s.users.Create(user)
	if err != nil {
		return nil, err
	}

	resp.Success = true
	resp.User = &user
	return resp, nil
}

type UserLoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type UserLoginResponse struct {
	Success bool          `json:"success"`
	Errors  []string      `json:"errors"`
	User    *sendkey.User `json:"user"`
}

func (s *UserService) Login(req UserLoginRequest) (*UserLoginResponse, error) {
	resp := &UserLoginResponse{}
	if req.Email == "" {
		resp.Errors = append(resp.Errors, "An email is required.")
	}
	if req.Password == "" {
		resp.Errors = append(resp.Errors, "A password is required.")
	}
	if len(resp.Errors) > 0 {
		resp.Success = false
		return resp, nil
	}

	user, err := s.users.FindByEmail(req.Email)
	if err != nil {
		return nil, err
	}
	if user == nil {
		resp.Errors = append(resp.Errors, "No user could be found with the specified email.")
		resp.Success = false
		return resp, nil
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password))
	if err != nil {
		if err != bcrypt.ErrMismatchedHashAndPassword {
			return nil, err
		}

		resp.Errors = append(resp.Errors, "The specified password is invalid.")
		resp.Success = false
		return resp, nil
	}

	resp.User = user
	resp.Success = true
	return resp, nil
}

func (s *UserService) FindUser(id uuid.UUID) (*sendkey.User, error) {
	return s.users.Find(id)
}
