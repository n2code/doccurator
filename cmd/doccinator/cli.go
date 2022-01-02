package main

import (
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/n2code/doccinator"
)

type CliRequest struct {
	verbose bool
	action  string
	targets []string
}

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
	case "add":
		if len(request.targets) < 1 {
			err = errors.New("No targets given!")
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
	case "demo-setup":
		doccinator.InitAppLibrary("/tmp")
		os.WriteFile("/tmp/.doccinator", []byte("file:///tmp/doccinator.db"), fs.ModePerm)
		os.WriteFile("/tmp/demofileA", []byte("hello world"), fs.ModePerm)
		os.WriteFile("/tmp/demofileB", []byte("goodbye!"), fs.ModePerm)
		doccinator.CreateDatabase("/tmp/doccinator.db")
	case "demo-list":
		err = doccinator.DiscoverAppLibrary("/tmp")
		if err != nil {
			return
		}
		doccinator.CommandList(os.Stdout)
	case "demo-scan":
		err = doccinator.DiscoverAppLibrary("/tmp")
		if err != nil {
			return
		}
		doccinator.CommandScan(os.Stdout)
	case "demo-scenario":
		err = doccinator.DiscoverAppLibrary("/tmp")
		if err != nil {
			return
		}
		err = doccinator.CommandAdd(23, "/tmp/demofileA")
		if err != nil {
			return
		}
		err = doccinator.CommandAdd(42, "/tmp/demofileB")
		if err != nil {
			return
		}
		doccinator.PersistDatabase()
	case "add":
		err = doccinator.DiscoverAppLibrary(workingDir)
		if err != nil {
			return
		}
		for _, target := range rq.targets {
			err = doccinator.CommandAdd(99, mustAbsPath(target))
			if err != nil {
				return
			}
		}
		doccinator.PersistDatabase()
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
