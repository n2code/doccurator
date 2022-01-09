package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"

	"github.com/n2code/doccinator"
	"github.com/n2code/doccinator/internal/document"
	"github.com/n2code/ndocid"
)

type CliRequest struct {
	verbose    bool
	quiet      bool
	action     string
	actionArgs []string
}

const idPattern = string(`[2-9]{5}[23456789ABCDEFHIJKLMNOPQRTUVWXYZ]+`)
const defaultDbFileName = string(`doccinator.db`)

//represents file.ext.23456X777.ndoc.ext or file_without_ext.23456X777.ndoc or .23456X777.ndoc.ext_only
var ndocFileNameRegex = regexp.MustCompile(`^.*\.(` + idPattern + `)\.ndoc(?:\.[^.]+)?$`)

func parseFlags(args []string, out io.Writer, errOut io.Writer) (request *CliRequest, exitCode int) {
	flags := flag.NewFlagSet("", flag.ExitOnError)
	flags.Usage = func() {
		flags.Output().Write([]byte("Usage: doccinator [MODE] <action> [TARGET...]\n\n MODE switches for all actions:\n"))
		flags.PrintDefaults()
	}

	request = &CliRequest{}
	var generalHelpRequested bool
	flags.BoolVar(&request.verbose, "v", false, "Output more details on what is done (verbose mode)")
	flags.BoolVar(&request.quiet, "q", false, "Output as little as possible, i.e. only requested information (quiet mode)")
	flags.BoolVar(&generalHelpRequested, "h", false, "Display general usage help")

	var err error
	defer func() {
		if err != nil {
			fmt.Fprintf(errOut, "%s\nUsage help: doccinator -h\n", err)
			exitCode = 2
			request = nil
		}
	}()

	flags.Parse(args) //exits on error

	if generalHelpRequested {
		flags.Usage()
		exitCode = 0
		request = nil
		return
	}
	if flags.NArg() == 0 {
		err = errors.New("No arguments given!")
		return
	}
	if request.verbose && request.quiet {
		err = errors.New("Quiet mode and verbose mode are mutually exclusive!")
		return
	}

	request.action = flags.Arg(0)
	request.actionArgs = flags.Args()[1:]

	switch request.action {
	case "add", "update", "status":
		if len(request.actionArgs) < 1 {
			err = errors.New("No targets given!")
			return
		}
	case "scan", "dump":
		if len(request.actionArgs) > 0 {
			err = errors.New("Too many arguments!")
			return
		}
	case "init":
		if len(request.actionArgs) != 1 {
			err = errors.New("Bad number of arguments, exactly one expected!")
			return
		}
	default:
		err = fmt.Errorf(`unknown action "%s"`, request.action)
		return
	}
	return
}

func (rq *CliRequest) execute() error {
	var config doccinator.CreateConfig
	if rq.verbose {
		config.Verbosity = doccinator.VerboseMode
	}
	if rq.quiet {
		config.Verbosity = doccinator.QuietMode
	}

	if rq.action == "init" {
		if _, err := doccinator.New(rq.actionArgs[0], filepath.Join(rq.actionArgs[0], defaultDbFileName), config); err != nil {
			return err
		}
	} else {
		workingDir, err := os.Getwd()
		if err != nil {
			return err
		}
		api, err := doccinator.Open(workingDir, config)
		if err != nil {
			return err
		}

		switch rq.action {
		case "dump":
			api.CommandDump()
		case "scan":
			if err := api.CommandScan(); err != nil {
				return err
			}
		case "add":
			for _, target := range rq.actionArgs {
				filename := filepath.Base(target)
				matches := ndocFileNameRegex.FindStringSubmatch(filename)
				if matches == nil {
					return fmt.Errorf(`ID missing in path %s`, target)
				}
				textId := matches[1]
				var numId uint64
				numId, err, _ = ndocid.Decode(textId)
				if err != nil {
					return fmt.Errorf(`bad ID in path %s (%w)`, target, err)
				}
				err = api.CommandAdd(document.DocumentId(numId), target)
				if err != nil {
					return err
				}
			}
			err = api.PersistChanges()
			if err != nil {
				return err
			}
		case "update":
			for _, target := range rq.actionArgs {
				err = api.CommandUpdateByPath(target)
				if err != nil {
					return err
				}
			}
			err = api.PersistChanges()
			if err != nil {
				return err
			}
		case "status":
			err = api.CommandStatus(rq.actionArgs)
			if err != nil {
				return err
			}
		default:
			panic("bad action")
		}
	}
	return nil
}

func main() {
	rq, rc := parseFlags(os.Args[1:], os.Stdout, os.Stderr)
	if rc != 0 || rq == nil {
		os.Exit(rc)
	}
	if err := rq.execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		switch rq.action {
		case "add", "update":
			if !rq.quiet {
				fmt.Fprintln(os.Stderr, "(library not modified because of errors)")
			}
		}
		os.Exit(1)
	}
	os.Exit(0)
}
