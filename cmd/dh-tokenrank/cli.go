package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type cliOptions struct {
	Home, State, Ticket, TicketFile string
	Install, All, Reconnect, JSON   bool
	Grants                          grantsFlag
}

var commandUsage = map[string]string{
	"setup":    "setup [--ticket-file FILE | DH_TICKET environment] --all [--install] [--reconnect] [--grant source=absolute-path] [--state-dir DIR]",
	"discover": "discover [--json] (lists supported local tool directories without reading conversations)",
	"status":   "status [--state-dir DIR] [--json] (always emits JSON)",
	"diagnose": "diagnose [--state-dir DIR] [--json] (version, writable state, source file counts, queue, scheduler, last error; no upload)",
	"doctor":   "doctor [--state-dir DIR] [--json] (alias of diagnose)",
	"repair":   "repair [--state-dir DIR] (strictly verify server-confirmed history; no re-upload)",
	"sync":     "sync [--state-dir DIR]", "periodic": "periodic [--state-dir DIR]",
	"uninstall": "uninstall [--state-dir DIR] (remove owned task; preserve local data)",
	"version":   "version", "privacy": "privacy (prints collection and credential policy)",
}

func printHelp(command string) {
	if command != "" {
		fmt.Println("dh-tokenrank " + commandUsage[command])
		return
	}
	fmt.Println("DH燃元榜", version, "\nCommands: setup, discover, status, diagnose, doctor, privacy, repair, sync, periodic, uninstall, version\nUse COMMAND --help for command-specific flags. Destination: "+serverURL)
}
func parseCLI(args []string) (string, cliOptions, bool, error) {
	var opt cliOptions
	if len(args) == 0 || args[0] == "help" {
		if len(args) > 2 {
			return "", opt, false, errors.New("help accepts at most one command")
		}
		if len(args) == 2 {
			if _, ok := commandUsage[args[1]]; !ok {
				return "", opt, false, errors.New("unknown command; use help")
			}
			printHelp(args[1])
		} else {
			printHelp("")
		}
		return "", opt, true, nil
	}
	command := args[0]
	if _, ok := commandUsage[command]; !ok {
		return command, opt, false, errors.New("unknown command; use help")
	}
	if len(args) == 2 && (args[1] == "--help" || args[1] == "-h") {
		printHelp(command)
		return command, opt, true, nil
	}
	if command == "version" || command == "privacy" {
		if len(args) != 1 {
			return command, opt, false, errors.New("unexpected arguments; use command --help")
		}
		if command == "version" {
			fmt.Println("DH燃元榜", version)
		} else {
			fmt.Println("DH reads only authorized current-user tool usage counters, timestamps, tools and model identifiers. It never uploads prompts, conversation bodies or absolute local paths. Destination: " + serverURL + ". Device credentials are stored in the private local state directory; uninstall preserves state. GitHub login makes profile and usage public by default; visibility can be withdrawn on the website.")
		}
		return command, opt, true, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return command, opt, false, err
	}
	opt.Home = home
	opt.State = filepath.Join(home, ".dh-tokenrank")
	fs := flag.NewFlagSet(command, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.Usage = func() { printHelp(command); fs.PrintDefaults() }
	if command != "discover" {
		fs.StringVar(&opt.State, "state-dir", opt.State, "local DH state directory")
	}
	if command == "setup" {
		fs.BoolVar(&opt.Install, "install", false, "install a per-user 30-minute task after local preflight")
		fs.BoolVar(&opt.All, "all", false, "authorize all supported current-user local tool directories")
		fs.BoolVar(&opt.Reconnect, "reconnect", false, "rebind existing state to the same account with a fresh setup code")
		fs.StringVar(&opt.Ticket, "ticket", "", "one-time setup code (prefer DH_TICKET or --ticket-file)")
		fs.StringVar(&opt.TicketFile, "ticket-file", "", "read a one-time setup code from a private UTF-8 file")
		fs.Var(&opt.Grants, "grant", "source=absolute-path read grant; repeatable")
	}
	if command == "diagnose" || command == "doctor" || command == "status" || command == "discover" {
		fs.BoolVar(&opt.JSON, "json", false, "machine-readable JSON output")
	}
	if err = fs.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return command, opt, true, nil
		}
		return command, opt, false, err
	}
	if fs.NArg() != 0 {
		return command, opt, false, errors.New("unexpected arguments")
	}
	if command == "setup" {
		fromEnv := strings.TrimSpace(os.Getenv("DH_TICKET"))
		provided := 0
		for _, v := range []string{opt.Ticket, opt.TicketFile, fromEnv} {
			if v != "" {
				provided++
			}
		}
		if provided > 1 {
			return command, opt, false, errors.New("choose only one of --ticket, --ticket-file or DH_TICKET")
		}
		if opt.TicketFile != "" {
			info, e := os.Stat(opt.TicketFile)
			if e != nil {
				return command, opt, false, fmt.Errorf("read setup-code file: %w", e)
			}
			if !info.Mode().IsRegular() || info.Size() > 4096 {
				return command, opt, false, errors.New("setup-code file must be a small regular UTF-8 file")
			}
			raw, e := os.ReadFile(opt.TicketFile)
			if e != nil {
				return command, opt, false, fmt.Errorf("read setup-code file: %w", e)
			}
			opt.Ticket = strings.TrimSpace(strings.TrimPrefix(string(raw), "\ufeff"))
		} else if opt.Ticket == "" {
			opt.Ticket = fromEnv
		}
		if opt.Reconnect && opt.Ticket == "" {
			return command, opt, false, errors.New("--reconnect requires a fresh code via DH_TICKET, --ticket-file or --ticket")
		}
	}
	if command == "doctor" {
		command = "diagnose"
	}
	return command, opt, false, nil
}
