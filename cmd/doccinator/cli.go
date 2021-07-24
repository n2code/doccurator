package main

import (
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"strings"

	. "github.com/n2code/doccinator"
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

func main() {
	rq, output, err, rc := parseFlags(os.Args[1:])
	if err != nil {
		fmt.Println(output)
		os.Exit(rc)
	}

	switch rq.action {
	case "demo-setup":
		os.Chdir("/tmp")
		InitAppLibrary()
		os.WriteFile("/tmp/.doccinator", []byte("file:///tmp/doccinator.db"), fs.ModePerm)
		os.WriteFile("/tmp/demofileA", []byte("hello world"), fs.ModePerm)
		os.WriteFile("/tmp/demofileB", []byte("goodbye!"), fs.ModePerm)
		os.WriteFile("/tmp/doccinator.db", []byte{}, fs.ModePerm)
		PersistDatabase()
	case "demo-list":
		os.Chdir("/tmp")
		found := DiscoverAppLibrary()
		if !found {
			fmt.Println("library not found")
			os.Exit(1)
		}
		CommandList()
	case "demo-scenario":
		os.Chdir("/tmp")
		found := DiscoverAppLibrary()
		if !found {
			fmt.Println("library not found")
			os.Exit(1)
		}
		CommandAdd(23, "/tmp/demofileA")
		CommandAdd(42, "/tmp/demofileB")
		PersistDatabase()
	}
}
