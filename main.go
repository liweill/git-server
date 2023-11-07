package main

import (
	cmd "git-server/internal/cmd/doc"
	"log"

	"github.com/urfave/cli"
	"os"
)

func main() {
	app := cli.NewApp()
	app.Name = "Gogs"
	app.Usage = "A painless self-hosted Git service"
	app.Version = "0.14.0+dev"
	app.Commands = []cli.Command{
		cmd.Hook,
		cmd.Web,
	}
	if err := app.Run(os.Args); err != nil {
		log.Fatal("Failed to start application: %v", err)
	}
}
