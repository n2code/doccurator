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
			output = fmt.Sprintln(err, "\n\n", output)
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
		doccinator.CommandList()
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
		doccinator.PersistDatabase("/tmp/doccinator.db")
	default:
		err = fmt.Errorf(`unknown action "%s"`, rq.action)
	}
	return
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
