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
	}
	return
}

func getRealWorkingDirectory() string {
	workingDirectory, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	absoluteWorkingDirectory, err := filepath.Abs(workingDirectory)
	if err != nil {
		panic(err)
	}
	realWorkingDirectory, err := filepath.EvalSymlinks(absoluteWorkingDirectory)
	if err != nil {
		panic(err)
	}
	return realWorkingDirectory
}

func (rq *CliRequest) execute() (err error) {
	var workingDir string
	workingDir, err = os.Getwd()
	if err != nil {
		return
	}

	switch rq.action {
	case "init":
		libRoot := mustAbsPath(rq.targets[0])
		doccinator.InitLibrary(libRoot, filepath.Join(libRoot, defaultDbFileName))
	case "dump":
		err = doccinator.DiscoverAppLibrary(workingDir)
		if err != nil {
			return
		}
		doccinator.CommandDump(os.Stdout)
	case "scan":
		err = doccinator.DiscoverAppLibrary(workingDir)
		if err != nil {
			return
		}
		doccinator.CommandScan(os.Stdout)
	case "add":
		err = doccinator.DiscoverAppLibrary(workingDir)
		if err != nil {
			return
		}
		for _, target := range rq.targets {
			filename := filepath.Base(target)
			matches := ndocFileNameRegex.FindStringSubmatch(filename)
			if matches == nil {
				err = fmt.Errorf(`ID missing in path %s`, target)
				return
			}
			textId := matches[1]
			var numId uint64
			numId, err, _ = ndocid.Decode(textId)
			if err != nil {
				err = fmt.Errorf(`bad ID in path %s (%s)`, target, err)
				return
			}
			err = doccinator.CommandAdd(document.DocumentId(numId), mustAbsPath(target))
			if err != nil {
				return
			}
		}
		doccinator.PersistDatabase()
	case "status":
		err = doccinator.DiscoverAppLibrary(workingDir)
		if err != nil {
			return
		}
		err = doccinator.CommandStatus(rq.targets, os.Stdout)
		if err != nil {
			return
		}
	default:
		err = fmt.Errorf(`unknown action "%s"`, rq.action)
	}
	return
}

func mustAbsPath(somePath string) string {
	abs, err := filepath.Abs(somePath)
	if err != nil {
		panic(err)
	}
	return abs
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
