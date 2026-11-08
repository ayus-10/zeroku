package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"

	"github.com/ayus-10/zeroku/internal/api"
	"github.com/ayus-10/zeroku/internal/awsinfra"
	"github.com/ayus-10/zeroku/internal/cli"
	"github.com/ayus-10/zeroku/internal/client"
	"github.com/ayus-10/zeroku/internal/config"
	"github.com/ayus-10/zeroku/internal/state"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	var err error

	switch os.Args[1] {
	case "init":
		err = initCmd(os.Args[2:])
	case "destroy":
		err = destroyCmd()
	case "deploy":
		err = deployCmd(os.Args[2:])
	case "get":
		err = getCmd(os.Args[2:])
	case "list":
		err = listCmd()
	case "stop":
		err = stopCmd(os.Args[2:])
	default:
		usage()
		os.Exit(2)
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, "error: ", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: zeroku <init|deploy|destroy|list|get> [flags]")
}

func initCmd(args []string) error {
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	region := fs.String("region", "ap-southeast-2", "AWS region")
	fs.Parse(args)

	ok, err := cli.Exists()
	if err != nil {
		return err
	}
	if ok {
		return errors.New("already initialized: run `zeroku destroy` first")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	infra, err := awsinfra.New(ctx, *region)
	if err != nil {
		return nil
	}

	dir, err := cli.Dir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}

	res, err := infra.Init(ctx, dir)
	if err != nil {
		return err
	}
	if err := cli.Save(res); err != nil {
		return err
	}

	fmt.Println("initialized: ", res.InstanceID)
	return nil
}

func destroyCmd() error {
	res, err := cli.Load()
	if errors.Is(err, os.ErrNotExist) {
		return errors.New("not initialized: run `zeroku init` first")
	}
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	infra, err := awsinfra.New(ctx, res.Region)
	if err != nil {
		return err
	}

	infra.Destroy(ctx, res)
	return cli.Remove()
}

func deployCmd(args []string) error {
	fs := flag.NewFlagSet("deploy", flag.ContinueOnError)

	gitRef := fs.String("ref", "main", "Git branch, tag or commit")
	port := fs.Int("port", 8080, "Container port")

	if err := fs.Parse(args); err != nil {
		return err
	}

	remaining := fs.Args()
	if len(remaining) != 2 {
		return fmt.Errorf("usage: zeroku deploy <appname> <giturl> [--ref main] [--port 1234]")
	}

	appName := remaining[0]
	gitURL := remaining[1]

	url := config.BaseURL()
	c := client.New(url)
	if err := c.Deploy(appName, api.DeployRequest{Repo: gitURL, GitRef: *gitRef, ContainerPort: *port}); err != nil {
		return err
	}

	return nil
}

func getCmd(args []string) error {
	fs := flag.NewFlagSet("get", flag.ContinueOnError)

	appName := fs.String("name", "", "App name")
	if err := fs.Parse(args); err != nil {
		return err
	}

	remaining := fs.Args()
	if len(remaining) != 0 {
		return fmt.Errorf("usage: zeroku get [--name <appname>]")
	}

	url := config.BaseURL()
	c := client.New(url)
	app, err := c.GetApp(*appName)
	if err != nil {
		return err
	}

	s, err := prettyPrintAppDetails(app)
	if err != nil {
		return err
	}
	fmt.Println(s)

	return nil
}

func listCmd() error {
	url := config.BaseURL()
	c := client.New(url)
	apps, err := c.ListApps()
	if err != nil {
		return err
	}

	for _, app := range apps {
		s, err := prettyPrintAppSummary(&app)
		if err != nil {
			return err
		}
		fmt.Println(s)
	}

	return nil
}

func stopCmd(args []string) error {
	fs := flag.NewFlagSet("stop", flag.ContinueOnError)

	appName := fs.String("name", "", "App name")
	if err := fs.Parse(args); err != nil {
		return err
	}

	remaining := fs.Args()
	if len(remaining) != 0 {
		return fmt.Errorf("usage: zeroku stop [--name <appname>]")
	}

	url := config.BaseURL()
	c := client.New(url)
	if err := c.Stop(*appName); err != nil {
		return err
	}

	return nil
}

func prettyPrintAppSummary(app *state.AppSummary) (string, error) {
	b, err := json.MarshalIndent(app, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func prettyPrintAppDetails(app *state.AppDetails) (string, error) {
	b, err := json.MarshalIndent(app, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}
