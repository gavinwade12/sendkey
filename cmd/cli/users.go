package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/gavinwade12/sendkey/pkg/client"
	"github.com/urfave/cli/v2"
)

func mountUserCommands(cliApp *cli.App) {
	cliApp.Commands = append(cliApp.Commands,
		createUserCommand,
		loginCommand,
	)
}

var createUserCommand = &cli.Command{
	Name:    "create_user",
	Aliases: []string{"cu"},
	Usage:   "Create a new sendkey user.",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "firstName",
			Aliases: []string{"f"},
			Usage:   "The user's first name.",
		},
		&cli.StringFlag{
			Name:    "lastName",
			Aliases: []string{"l"},
			Usage:   "The user's last name.",
		},
		&cli.StringFlag{
			Name:     "email",
			Aliases:  []string{"e"},
			Usage:    "The user's email.",
			Required: true,
		},
		&cli.StringFlag{
			Name:     "password",
			Aliases:  []string{"p"},
			Usage:    "The user's password.",
			Required: true,
		},
	},
	Action: func(ctx *cli.Context) error {
		err := ensureClient(ctx.String("config"))
		if err != nil {
			return err
		}

		req := client.CreateUserRequest{
			FirstName: ctx.String("firstName"),
			LastName:  ctx.String("lastName"),
			Email:     ctx.String("email"),
			Password:  ctx.String("password"),
		}

		res, e, err := sendkeyClient.Users.CreateUser(req)
		if err != nil {
			return err
		}
		if e != nil {
			return fmt.Errorf("[%d]: %s", e.StatusCode, e.Message)
		}
		if !res.Success {
			return fmt.Errorf(strings.Join(res.Errors, "; "))
		}

		fmt.Println("Successfully created user:")
		fmt.Printf("\tID: %s\n", res.User.ID.String())
		fmt.Printf("\tFirstName: %s\n", res.User.FirstName)
		fmt.Printf("\tLastName: %s\n", res.User.LastName)
		fmt.Printf("\tEmail: %s\n", res.User.Email)
		fmt.Printf("\tEmailVerified: %s\n", strconv.FormatBool(res.User.EmailVerified))
		fmt.Printf("\tCreatedAtUtc: %s\n", res.User.CreatedAtUTC.String())

		return nil
	},
}

var loginCommand = &cli.Command{
	Name:  "login",
	Usage: "Login as a sendkey user.",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:     "email",
			Aliases:  []string{"e"},
			Usage:    "The user's email.",
			Required: true,
		},
		&cli.StringFlag{
			Name:     "password",
			Aliases:  []string{"p"},
			Usage:    "The user's password.",
			Required: true,
		},
	},
	Action: func(ctx *cli.Context) error {
		err := ensureClient(ctx.String("config"))
		if err != nil {
			return err
		}

		res, e, err := sendkeyClient.Users.Login(ctx.String("email"), ctx.String("password"))
		if err != nil {
			return err
		}
		if e != nil {
			return fmt.Errorf("[%d]: %s", e.StatusCode, e.Message)
		}
		if !res.Success {
			return fmt.Errorf(strings.Join(res.Errors, "; "))
		}

		session, err := loadSession()
		if err != nil {
			return err
		}

		session.UserID = res.User.ID
		session.AccessToken = Token{
			Token:   res.AccessToken.Token,
			Expires: res.AccessToken.Expires,
		}
		session.RefreshToken = Token{
			Token:   res.RefreshToken.Token,
			Expires: res.RefreshToken.Expires,
		}
		return saveSession(*session)
	},
}
