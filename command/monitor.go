package command

import (
	"fmt"
	"github.com/mitchellh/cli"
	"github.com/posener/complete"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

var _ cli.Command = (*MonitorCommand)(nil)
var _ cli.CommandAutocomplete = (*MonitorCommand)(nil)

type MonitorCommand struct {
	*BaseCommand

	logLevel string
}

func (c *MonitorCommand) Synopsis() string {
	return "Stream log messages from a Vault server"
}

func (c *MonitorCommand) Help() string {
	helpText := `
Usage: vault monitor [options]

	Stream log messages of a Vault server. The monitor command lets you listen
	for log levels that may be filtered out of the server logs. For example,
	the server may be logging at the INFO level, but with the monitor command
	you can set -log-level DEBUG.
` + c.Flags().Help()

	return strings.TrimSpace(helpText)
}

func (c *MonitorCommand) Flags() *FlagSets {
	set := c.flagSet(FlagSetHTTP)

	f := set.NewFlagSet("Monitor Options")
	f.StringVar(&StringVar{
		Name:       "log-level",
		Target:     &c.logLevel,
		Default:    "INFO",
		Completion: complete.PredictSet("TRACE", "DEBUG", "INFO", "WARN", "ERROR"),
		Usage: "If passed, the log level to monitor logs. Supported values" +
			"(in order of detail) are \"TRACE\", \"DEBUG\", \"INFO\", \"WARN\"" +
			" and \"ERROR\".",
	})

	return set
}

func (c *MonitorCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (c *MonitorCommand) AutocompleteFlags() complete.Flags {
	return c.Flags().Completions()
}

func (c *MonitorCommand) Run(args []string) int {
	if err := c.Flags().Parse(args); err != nil {
		c.UI.Error(err.Error())
		return 1
	}

	client, err := c.Client()
	if err != nil {
		c.UI.Error(err.Error())
		return 2
	}

	var logCh chan string
	stopCh := make(chan struct{})
	defer close(stopCh)

START:
	logCh, err = client.Sys().Monitor(c.logLevel, stopCh)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error starting monitor: %s", err))
		return 1
	}

	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, os.Interrupt, syscall.SIGTERM)

	go func() {
	OUTER:
		for {
			select {
			case log := <-logCh:
				if log == "" {
					break OUTER
				}
				c.UI.Info(log)
			case <-stopCh:
				return
			}
		}
	}()

	select {
	case <-signalCh:
		return 0
	case <- stopCh:
		goto START
	}

	return 0
}
