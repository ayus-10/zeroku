package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"

	"github.com/ayus-10/zeroku/internal/awsinfra"
	"github.com/ayus-10/zeroku/internal/cli"
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
	fmt.Fprintln(os.Stderr, "usage: zeroku <init|deploy|destroy> [flags]")
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
