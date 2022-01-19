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
	verbose     bool
	quiet       bool
	action      string
	actionFlags map[string]interface{}
	actionArgs  []string
}

const idPattern = string(`[2-9]{5}[23456789ABCDEFHIJKLMNOPQRTUVWXYZ]+`)
const defaultDbFileName = string(`doccinator.db`)

//represents file.ext.23456X777.ndoc.ext or file_without_ext.23456X777.ndoc or .23456X777.ndoc.ext_only
var ndocFileNameRegex = regexp.MustCompile(`^.*\.(` + idPattern + `)\.ndoc(?:\.[^.]+)?$`)

func parseFlags(args []string, out io.Writer, errOut io.Writer) (request *CliRequest, exitCode int) {
	flags := flag.NewFlagSet("", flag.ExitOnError)
	flags.Usage = func() {
		flags.Output().Write([]byte(`
Usage:
   doccinator [-v|-q|-h] <ACTION> [FLAG] [TARGET]

 ACTIONs:  init  status  add  update  retire  forget  tree  dump

`))
		flags.PrintDefaults()
		flags.Output().Write([]byte(`
 FLAG(s) and TARGET(s) are action-specific.
 You can read the help on any action:
    doccinator <ACTION> -h

`))

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
	request.actionFlags = make(map[string]interface{})
	request.actionArgs = flags.Args()[1:]
	actionDescriptionIndent := "  "
	actionDescription := actionDescriptionIndent
	flagSpecification := ""
	argumentSpecification := ""

	actionParams := flag.NewFlagSet(request.action+" action", flag.ExitOnError)
	actionParams.Usage = func() {
		fmt.Fprintf(actionParams.Output(), `
Usage of %s action:
   doccinator [MODE] %s%s%s

%s
`, request.action, request.action, flagSpecification, argumentSpecification, actionDescription)
		if len(flagSpecification) > 0 {
			fmt.Fprint(actionParams.Output(), `
 Available flags:
`)
		}
		actionParams.PrintDefaults()
		fmt.Fprintf(actionParams.Output(), `
 Global MODE documentation can be shown by:
    doccinator -h

`)
	}

	switch request.action {
	case "status":
		argumentSpecification = " [FILEPATH...]"
		actionDescription += "Compare files in the library folder to the records.\n" +
			actionDescriptionIndent + "If one or more FILEPATHs are given only compare those files otherwise\n" +
			actionDescriptionIndent + "compare all. For an explicit list of paths all states are listed. For\n" +
			actionDescriptionIndent + "a full scan (no paths specified) unchanged tracked files are omitted."
		actionParams.Parse(request.actionArgs)
		request.actionArgs = actionParams.Args()
		//beyond flags all arguments are optional
	case "add", "update", "retire", "forget":
		argumentSpecification = " FILEPATH..."
		switch request.action {
		case "add":
			actionDescription += "Add the file(s) at the given FILEPATH(s) to the library records."
		case "update":
			actionDescription += "Update the library records to match the current state of the file(s)\n" +
				actionDescriptionIndent + "at the given FILEPATH(s)."
		case "retire":
			actionDescription += "Mark the library records corresponding to the given FILEPATH(s) as\n" +
				actionDescriptionIndent + "obsolete. The real file is expected to be removed manually.\n" +
				actionDescriptionIndent + "If an identical file appears at a later point in time the library is\n" +
				actionDescriptionIndent + "thereby able to recognize it as an obsolete duplicate (\"zombie\")."
		case "forget":
			flagSpecification = " [-no-require-retired]"
			actionDescription += "Delete the library records corresponding to the given FILEPATH(s).\n" +
				actionDescriptionIndent + "The paths have to be retired unless the flag to ignore this is set."
			request.actionFlags["no-require-retired"] = actionParams.Bool("no-require-retired", false, "ignore if a document is retired or not, force forget regardless")
		}
		actionParams.Parse(request.actionArgs)
		request.actionArgs = actionParams.Args()
		if actionParams.NArg() < 1 {
			err = errors.New("no targets given")
			return
		}
	case "dump":
		flagSpecification = " [-exclude-retired]"
		actionDescription += "Print all library records."
		request.actionFlags["exclude-retired"] = actionParams.Bool("exclude-retired", false, "do not print records marked as obsolete (\"retired\")")
		actionParams.Parse(request.actionArgs)
		request.actionArgs = actionParams.Args()
		if actionParams.NArg() > 0 {
			err = errors.New("too many arguments")
			return
		}
	case "tree":
		flagSpecification = " [-diff]"
		actionDescription += "Display the library as a tree which represents the union of all\n" +
			actionDescriptionIndent + "library records and the files currently present in the library folder."
		request.actionFlags["diff"] = actionParams.Bool("diff", false, "show only difference to library records, i.e. exclude\nunchanged tracked and tracked-as-removed files")
		actionParams.Parse(request.actionArgs)
		request.actionArgs = actionParams.Args()
		if actionParams.NArg() > 0 {
			err = errors.New("command accepts no arguments, only flags")
			return
		}
	case "init":
		argumentSpecification = " DIRECTORY"
		actionDescription += "Initialize a new library in the given root DIRECTORY. Everything below\n" +
			actionDescriptionIndent + "the root is considered to be located 'inside' the library. All files\n" +
			actionDescriptionIndent + "inside can be recorded for the purpose of change detection."
		actionParams.Parse(request.actionArgs)
		request.actionArgs = actionParams.Args()
		if actionParams.NArg() != 1 {
			err = errors.New("bad number of arguments, exactly one expected")
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
		workingDir, _ := os.Getwd()
		api, err := doccinator.Open(workingDir, config)
		if err != nil {
			return err
		}

		switch rq.action {
		case "dump":
			api.CommandDump(*(rq.actionFlags["exclude-retired"].(*bool)))
		case "tree":
			if err := api.CommandTree(*(rq.actionFlags["diff"].(*bool))); err != nil {
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
		case "retire":
			for _, target := range rq.actionArgs {
				err = api.CommandRetireByPath(target)
				if err != nil {
					return err
				}
			}
			err = api.PersistChanges()
			if err != nil {
				return err
			}
		case "forget":
			for _, target := range rq.actionArgs {
				err = api.CommandForgetByPath(target, *(rq.actionFlags["no-require-retired"].(*bool)))
				if err != nil {
					return err
				}
			}
			err = api.PersistChanges()
			if err != nil {
				return err
			}
		case "status":
			if err = api.CommandStatus(rq.actionArgs); err != nil {
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
