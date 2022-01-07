package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/n2code/doccinator"
	"github.com/n2code/doccinator/internal/document"
	"github.com/n2code/ndocid"
)

type CliRequest struct {
	verbose bool
	action  string
	targets []string
}

const idPattern = string(`[2-9]{5}[23456789ABCDEFHIJKLMNOPQRTUVWXYZ]+`)
const defaultDbFileName = string(`doccinator.db`)

//represents file.ext.23456X777.ndoc.ext or file_without_ext.23456X777.ndoc or .23456X777.ndoc.ext_only
var ndocFileNameRegex = regexp.MustCompile(`^.*\.(` + idPattern + `)\.ndoc(?:\.[^.]+)?$`)

func parseFlags(args []string) (request *CliRequest, output string, err error, exitCode int) {
	flags := flag.NewFlagSet("", flag.ContinueOnError)

	var outputBuffer strings.Builder
	flags.SetOutput(&outputBuffer)

	flags.Usage = func() {
		outputBuffer.WriteString("Usage: doccinator <action> [FLAGS...] FILE...\n\n Flags:\n")
		flags.PrintDefaults()
	}

	request = &CliRequest{}
	flags.BoolVar(&request.verbose, "v", false, "Output more details on what is done (verbose mode)")

	defer func() {
		output = outputBuffer.String()
		if err == flag.ErrHelp {
			exitCode = 0
		} else if err != nil {
			if len(output) > 0 {
				output = fmt.Sprint(err, "\n\n", output)
			} else {
				output = fmt.Sprint(err)
			}

			exitCode = 2
		}
	}()

	err = flags.Parse(args)
	if err != nil {
		return
	}

	if flags.NArg() == 0 {
		err = errors.New("No arguments given!")
		flags.Usage()
		return
	}
	if flags.Arg(0) == "help" {
		flags.Usage()
		err = flag.ErrHelp
		return
	}

	request.action = flags.Arg(0)
	request.targets = flags.Args()[1:]

	switch request.action {
	case "add", "status":
		if len(request.targets) < 1 {
			err = errors.New("No targets given!")
			return
		}
	case "scan", "dump":
		if len(request.targets) > 0 {
			err = errors.New("Too many arguments!")
			return
		}
	case "init":
		if len(request.targets) != 1 {
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
	if rq.action == "init" {
		if _, err := doccinator.New(rq.targets[0], filepath.Join(rq.targets[0], defaultDbFileName)); err != nil {
			return err
		}
	} else {
		workingDir, err := os.Getwd()
		if err != nil {
			return err
		}
		api, err := doccinator.Open(workingDir)
		if err != nil {
			return err
		}
		switch rq.action {
		case "dump":
			api.CommandDump()
		case "scan":
			api.CommandScan()
		case "add":
			for _, target := range rq.targets {
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
		case "status":
			err = api.CommandStatus(rq.targets)
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
	rq, output, err, rc := parseFlags(os.Args[1:])
	if err != nil {
		fmt.Println(output)
		os.Exit(rc)
	}
	err = rq.execute()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(0)
}
