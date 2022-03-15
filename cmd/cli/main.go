package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path"

	"github.com/gavinwade12/sendkey/pkg/client"
	"github.com/google/uuid"
	"github.com/urfave/cli/v2"
)

var version string

var sendkeyClient *client.Client

type config struct {
	BaseURL string
}

var defaultConfig = config{
	BaseURL: `https://api.sendkey.me/v1`,
}

func main() {
	cliApp := &cli.App{
		Name:        "sendkey",
		Version:     version,
		Description: "A CLI tool for interfacing with the sendkey REST API.",
		Usage:       "Inteface with the sendkey API from the commandline.",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:      "config",
				Aliases:   []string{"c"},
				Usage:     "The path to a custom JSON config file to be used.",
				TakesFile: true,
				EnvVars:   []string{"SENDKEY_CLI_CONFIG", "SENDKEY_CONFIG"},
			},
		},
	}
	mountUserCommands(cliApp)
	mountEntryCommands(cliApp)

	cliApp.Setup()
	if err := cliApp.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func ensureClient(configFile string) error {
	if sendkeyClient != nil {
		return nil
	}

	var (
		cfg *config
		err error
	)
	if configFile != "" {
		cfg, err = readConfig(configFile)
		if err != nil {
			return err
		}
	} else {
		cfg = &defaultConfig
	}

	session, err := loadSession()
	if err != nil {
		return err
	}

	sendkeyClient = client.NewClient(cfg.BaseURL,
		client.WithDefaultHeaders(map[string][]string{
			"User-Agent": {"sendkey-cli@" + version},
		}),
		client.WithSession(session.UserID, session.RefreshToken.Token,
			session.AccessToken.Token),
	)

	return nil
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

func saveSession(session Session) error {
	b, err := json.Marshal(session)
	if err != nil {
		return err
	}

	homedir, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	fileName := path.Join(homedir, ".sendkey")

	file, err := os.OpenFile(fileName, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.ModePerm)
	if err != nil {
		return err
	}

	_, err = file.Write(b)
	return err
}

func loadSession() (*Session, error) {
	homedir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	fileName := path.Join(homedir, ".sendkey")
	b, err := os.ReadFile(fileName)
	if err != nil {
		if os.IsNotExist(err) {
			return &Session{}, nil
		}
		return nil, err
	}

	var session Session
	err = json.Unmarshal(b, &session)
	if err != nil {
		return nil, err
	}
	return &session, nil
}

type Token struct {
	Token   string
	Expires int64
}

type Session struct {
	UserID       uuid.UUID
	AccessToken  Token
	RefreshToken Token
}
