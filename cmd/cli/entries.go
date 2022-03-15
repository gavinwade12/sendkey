package main

import (
	"fmt"
	"strings"

	"github.com/gavinwade12/sendkey/pkg/client"
	"github.com/urfave/cli/v2"
)

func mountEntryCommands(cliApp *cli.App) {
	cliApp.Commands = append(cliApp.Commands,
		createEntryCommand,
		listEntriesCommand,
	)
}

var createEntryCommand = &cli.Command{
	Name:    "create_entry",
	Aliases: []string{"ce"},
	Usage:   "Create a new sendkey entry.",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:     "name",
			Aliases:  []string{"n"},
			Usage:    "The entry name.",
			Required: true,
		},
		&cli.IntFlag{
			Name:     "duration",
			Aliases:  []string{"d"},
			Usage:    "The duration (in minutes) the entry is valid.",
			Required: true,
		},
		&cli.StringFlag{
			Name:     "sendTo",
			Aliases:  []string{"st"},
			Usage:    "The email address to which the entry should be sent.",
			Required: true,
		},
		&cli.StringFlag{
			Name:     "value",
			Aliases:  []string{"v"},
			Usage:    "The entry value.",
			Required: true,
		},
		&cli.StringFlag{
			Name:     "secret",
			Aliases:  []string{"s"},
			Usage:    "The secret required to view the entry value.",
			Required: true,
		},
	},
	Action: func(ctx *cli.Context) error {
		err := ensureClient(ctx.String("config"))
		if err != nil {
			return err
		}

		req := client.CreateEntryRequest{
			Name:            ctx.String("name"),
			SendToEmail:     ctx.String("sendTo"),
			Value:           ctx.String("value"),
			Secret:          ctx.String("secret"),
			DurationMinutes: ctx.Int("duration"),
		}

		res, e, err := sendkeyClient.Entries.CreateEntry(req)
		if err != nil {
			return err
		}
		if e != nil {
			return fmt.Errorf("[%d]: %s", e.StatusCode, e.Message)
		}
		if !res.Success {
			return fmt.Errorf(strings.Join(res.Errors, "; "))
		}

		fmt.Println("Successfully created entry:")
		fmt.Printf("\tID: %s\n", res.Entry.ID.String())
		fmt.Printf("\tName: %s\n", res.Entry.Name)
		fmt.Printf("\tSentTo: %s\n", res.Entry.SentToEmail)
		fmt.Printf("\tCreatedAtUtc: %s\n", res.Entry.CreatedAtUTC.String())
		fmt.Printf("\tExpiresAtUtc: %s\n", res.Entry.ExpiresAtUTC.String())

		return nil
	},
}

var listEntriesCommand = &cli.Command{
	Name:    "list_entries",
	Aliases: []string{"le"},
	Usage:   "Lists all unclaimed, unexpired entries.",
	Action: func(ctx *cli.Context) error {
		err := ensureClient(ctx.String("config"))
		if err != nil {
			return err
		}

		res, e, err := sendkeyClient.Entries.ListEntries()
		if err != nil {
			return err
		}
		if e != nil {
			return fmt.Errorf("[%d]: %s", e.StatusCode, e.Message)
		}

		for _, entry := range res {
			fmt.Printf("ID: %s\n", entry.ID.String())
			fmt.Printf("\tName: %s\n", entry.Name)
			fmt.Printf("\tSentTo: %s\n", entry.SentToEmail)
			fmt.Printf("\tCreatedAtUtc: %s\n", entry.CreatedAtUTC.String())
			fmt.Printf("\tExpiresAtUtc: %s\n", entry.ExpiresAtUTC.String())
			fmt.Println()
		}

		return nil
	},
}
