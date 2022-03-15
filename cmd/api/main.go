package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gavinwade12/sendkey"
	"github.com/gavinwade12/sendkey/internal/app"
	"github.com/gavinwade12/sendkey/internal/mysql"
	"github.com/google/uuid"
	"github.com/julienschmidt/httprouter"
	"github.com/rs/cors"
)

type config struct {
	Key                string
	MaxInvalidAttempts int
	Host               string
	Port               string
	Cors               struct {
		AllowedOrigins []string
		AllowedMethods []string
		AllowedHeaders []string
	}
	Auth struct {
		SigningKey                string
		AccessTokenDurationMins   int
		RefreshTokenDurationHours int
	}
	MySQL struct {
		DSN           string
		MigrationsDir string
	}
}

func main() {
	configPath := flag.String("config", "config.json", "the path to the config file")
	flag.Parse()

	cfg, err := readConfig(*configPath)
	if err != nil {
		log.Fatal(err)
	}

	opts := []mysql.Option{mysql.AutoCreateDB()}
	if cfg.MySQL.MigrationsDir != "" {
		opts = append(opts, mysql.WithMigrations(cfg.MySQL.MigrationsDir))
	}
	db, err := mysql.NewDB(cfg.MySQL.DSN, opts...)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// TODO: create a transaction for each request? allow services to request a transaction?

	accessTokenLifetime := time.Minute * time.Duration(cfg.Auth.AccessTokenDurationMins)
	refreshTokenLifetime := time.Hour * time.Duration(cfg.Auth.RefreshTokenDurationHours)
	atm := newAuthTokenManager([]byte(cfg.Auth.SigningKey), accessTokenLifetime, refreshTokenLifetime)

	r := httprouter.New()
	setUserID := setUserID(atm)
	pipeline := func(a action) httprouter.Handle {
		return acceptJSON(cleanOutput(setUserID(a)))
	}

	bc := baseController{}

	userSvc := app.NewUserService(db.Users)
	uc := &UsersController{bc, userSvc, atm, db.RefreshTokens}

	entrySvc := app.NewEntryService(db.Entries, []byte(cfg.Key), cfg.MaxInvalidAttempts)
	ec := &EntriesController{bc, entrySvc}

	r.POST("/users", pipeline(uc.CreateUser))
	r.POST("/login", pipeline(uc.Login))
	r.POST("/token", pipeline(uc.RefreshToken))

	r.POST("/entries", pipeline(ec.CreateEntry))
	r.GET("/entries/:entryID", pipeline(ec.FindEntry))
	r.GET("/entries/:entryID/value", pipeline(ec.EntryValue))
	r.GET("/users/:userID/entries", pipeline(ec.FindUserEntries))

	c := cors.New(cors.Options{
		AllowedOrigins: cfg.Cors.AllowedOrigins,
		AllowedMethods: cfg.Cors.AllowedMethods,
		AllowedHeaders: cfg.Cors.AllowedHeaders,
	})

	addr := net.JoinHostPort(cfg.Host, cfg.Port)
	fmt.Printf("listening on %s\n", addr)
	if err = http.ListenAndServe(addr, c.Handler(r)); err != nil {
		log.Fatal(err)
	}
}

func acceptJSON(h httprouter.Handle) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		ct := r.Header.Get("Content-Type")
		if ct != "" && ct != "application/json" {
			http.Error(w, "Invalid Content-Type", http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")

		h(w, r, p)
	}
}

type action func(http.ResponseWriter, *http.Request, httprouter.Params) error

func cleanOutput(a action) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		defer func() {
			if r := recover(); r != nil {
				err, ok := r.(error)
				if !ok {
					err = fmt.Errorf("%v", r)
				}

				e := Error{StatusCode: http.StatusInternalServerError, Message: fmt.Sprintf("panic recovery: %v", err)}
				w.WriteHeader(e.StatusCode)
				json.NewEncoder(w).Encode(e)
				json.NewEncoder(log.Writer()).Encode(e)
			}
		}()

		err := a(w, r, p)
		if err == nil {
			return
		}

		var (
			e  Error
			ok bool
		)
		if e, ok = err.(Error); !ok {
			e = Error{StatusCode: http.StatusInternalServerError, Message: err.Error()}
		}

		w.WriteHeader(e.StatusCode)
		json.NewEncoder(w).Encode(e)
		json.NewEncoder(log.Writer()).Encode(e)
	}
}

// Error is an error returned from the API.
type Error struct {
	UserID     uuid.UUID `json:"userId"`
	StatusCode int       `json:"statusCode"`
	Message    string    `json:"message"`
}

func (e Error) Error() string {
	return e.Message
}

func readConfig(path string) (*config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening config file: %w", err)
	}
	defer f.Close()

	cfg := &config{}
	if err = json.NewDecoder(f).Decode(cfg); err != nil {
		return nil, fmt.Errorf("decoding config file: %w", err)
	}

	return cfg, nil
}

type userIDCtxKey string

const userIDCtxKeyValue = userIDCtxKey("userID")

func setUserID(atv AccessTokenVerifier) func(a action) action {
	return func(a action) action {
		return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) error {
			token := r.Header.Get("Authorization")
			if token == "" {
				return a(w, r, p)
			}
			token = strings.TrimPrefix(token, "Bearer ")

			userID, err := atv.Verify(token)
			if err != nil {
				return Error{StatusCode: http.StatusUnauthorized, Message: err.Error()}
			}

			ctx := r.Context()
			ctx = context.WithValue(ctx, userIDCtxKeyValue, userID)
			r = r.WithContext(ctx)

			return a(w, r, p)
		}
	}
}

type baseController struct {
}

func (c baseController) GetCurrentUserID(r *http.Request) (uuid.UUID, error) {
	userID := r.Context().Value(userIDCtxKeyValue)
	if userID == nil {
		return uuid.Nil, fmt.Errorf("unable to get current user id")
	}

	return userID.(uuid.UUID), nil
}

func (c baseController) GetCurrentUser(r *http.Request, us *app.UserService) (*sendkey.User, error) {
	id, err := c.GetCurrentUserID(r)
	if err != nil {
		return nil, err
	}

	return us.FindUser(id)
}
