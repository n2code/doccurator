package main

import (
	"errors"
	"flag"
	"fmt"
	"github.com/n2code/doccurator"
	"github.com/n2code/doccurator/internal/document"
	"github.com/n2code/ndocid"
	"io"
	"os"
	"path/filepath"
)

type CliRequest struct {
	verbose     bool
	quiet       bool
	action      string
	actionFlags map[string]interface{}
	actionArgs  []string
}

const defaultDbFileName = `doccurator.db`

func parseFlags(args []string, out io.Writer, errOut io.Writer) (request *CliRequest, exitCode int) {
	flags := flag.NewFlagSet("", flag.ExitOnError)
	flags.Usage = func() {
		flags.Output().Write([]byte(`
Usage:
   doccurator [-v|-q|-h] <ACTION> [FLAG] [TARGET]

 ACTIONs:  init  status  add  update  retire  forget  tree  dump

`))
		flags.PrintDefaults()
		flags.Output().Write([]byte(`
 FLAG(s) and TARGET(s) are action-specific.
 You can read the help on any action:
    doccurator <ACTION> -h

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
			fmt.Fprintf(errOut, "%s\nUsage help: doccurator -h\n", err)
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
   doccurator [MODE] %s%s%s

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
    doccurator -h

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
		argumentSpecification = " [FILEPATH...]"
		switch request.action {
		case "add":
			flagSpecification = " [-all-untracked] [-rename] [-force]"
			actionDescription += "Add the file(s) at the given FILEPATH(s) to the library records.\n" +
				actionDescriptionIndent + "Alternatively all untracked files can be added automatically via flag."
			request.actionFlags["all-untracked"] = actionParams.Bool("all-untracked", false, "add all untracked files anywhere inside the library\n"+
				"(requires *standardized* filenames to extract IDs)")
			request.actionFlags["force"] = actionParams.Bool("force", false, "allow adding even duplicates, moved, and obsolete files as new\n"+
				"(because this is likely undesired and thus blocked by default)")
			request.actionFlags["rename"] = actionParams.Bool("rename", false, "rename added files to standardized filename")
			request.actionFlags["id"] = actionParams.String("id", "", "specify new document ID instead of extracting it from filename\n"+
				"(only a single FILEPATH can be given, -all-untracked must not be used)\n"+
				"FORMAT 1: doccurator add -id 63835AEV9E my_document.pdf\n"+
				"FORMAT 2: doccurator add -id=55565IEV9E my_document.pdf")
		case "update":
			actionDescription += "Update the existing library records to match the current state of\n" +
				actionDescriptionIndent + "the file(s) at the given FILEPATH(s)."
		case "retire":
			actionDescription += "Mark the library records corresponding to the given FILEPATH(s) as\n" +
				actionDescriptionIndent + "obsolete. The real file is expected to be removed manually.\n" +
				actionDescriptionIndent + "If an identical file appears at a later point in time the library is\n" +
				actionDescriptionIndent + "thereby able to recognize it as an obsolete duplicate (\"zombie\")."
		case "forget":
			flagSpecification = " -all-retired |"
			argumentSpecification = " ID..."
			actionDescription += "Delete the library records corresponding to the given ID(s).\n" +
				actionDescriptionIndent + "Only retired documents can be forgotten."
			request.actionFlags["all-retired"] = actionParams.Bool("all-retired", false, "forget all retired documents")
		}
		actionParams.Parse(request.actionArgs)
		request.actionArgs = actionParams.Args()
		switch {
		case request.action == "add" && *(request.actionFlags["all-untracked"].(*bool)):
			if actionParams.NArg() != 0 {
				err = errors.New(`no FILEPATHs must be given when using flag "-all-untracked"`)
				return
			}
			if *(request.actionFlags["id"].(*string)) != "" {
				err = errors.New(`flag "-id" must not be used together with "-all-untracked"`)
				return
			}
		case request.action == "add" && *(request.actionFlags["id"].(*string)) != "":
			if actionParams.NArg() != 1 {
				err = errors.New(`exactly one FILEPATH must be given when using flag "-id"`)
				return
			}
		case request.action == "forget" && *(request.actionFlags["all-retired"].(*bool)):
			if actionParams.NArg() != 0 {
				err = errors.New(`no IDs must be given when using flag "-all-retired"`)
				return
			}
		default:
			if actionParams.NArg() < 1 {
				err = errors.New("no targets given")
				return
			}
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

func (rq *CliRequest) execute() (execErr error) {
	var config doccurator.CreateConfig
	if rq.verbose {
		config.Verbosity = doccurator.VerboseMode
	}
	if rq.quiet {
		config.Verbosity = doccurator.QuietMode
	}

	if rq.action == "init" {
		if _, err := doccurator.New(rq.actionArgs[0], filepath.Join(rq.actionArgs[0], defaultDbFileName), config); err != nil {
			return err
		}
	} else {
		workingDir, _ := os.Getwd()
		api, err := doccurator.Open(workingDir, config)
		if err != nil {
			return err
		}
		defer func() {
			if execErr != nil {
				rollbackErr := api.RollbackFilesystemChanges()
				if rollbackErr != nil {
					execErr = fmt.Errorf("%w (rollback attempt failed: %s)", execErr, rollbackErr)
				}
			}
		}()

		switch rq.action {
		case "dump":
			api.CommandDump(*(rq.actionFlags["exclude-retired"].(*bool)))
		case "tree":
			if err := api.CommandTree(*(rq.actionFlags["diff"].(*bool))); err != nil {
				return err
			}
		case "add":
			tryRename := *(rq.actionFlags["rename"].(*bool))
			forceIfDuplicateMovedOrObsolete := *(rq.actionFlags["force"].(*bool))
			var addedIds []document.Id
			if *(rq.actionFlags["all-untracked"].(*bool)) {
				var addErr error
				addedIds, addErr = api.CommandAddAllUntracked(forceIfDuplicateMovedOrObsolete)
				if addErr != nil {
					return addErr
				}
			} else {
				if explicitId := *(rq.actionFlags["id"].(*string)); explicitId != "" {
					numId, err, complete := ndocid.Decode(explicitId)
					if err != nil {
						return fmt.Errorf(`error in ID "%s" (%w)`, explicitId, err)
					}
					if !complete {
						return fmt.Errorf(`incomplete ID "%s"`, explicitId)
					}
					newId := document.Id(numId)
					err = api.CommandAddSingle(newId, rq.actionArgs[0], forceIfDuplicateMovedOrObsolete)
					if err != nil {
						return err
					}
					addedIds = append(addedIds, newId)
				} else {
					for _, target := range rq.actionArgs {
						newId, err := doccurator.ExtractIdFromStandardizedFilename(target)
						if err != nil {
							return fmt.Errorf(`bad path %s: (%w)`, target, err)
						}
						err = api.CommandAddSingle(newId, target, forceIfDuplicateMovedOrObsolete)
						if err != nil {
							return err
						}
						addedIds = append(addedIds, newId)
					}
				}
			}
			if tryRename {
				for _, addedId := range addedIds {
					err := api.CommandStandardizeLocation(addedId)
					if err != nil {
						return fmt.Errorf(`renaming file of document %s failed: %w`, addedId, err)
					}
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
			if *(rq.actionFlags["all-retired"].(*bool)) {
				api.CommandForgetAllObsolete()
			} else {
				for _, target := range rq.actionArgs {
					numId, err, complete := ndocid.Decode(target)
					if err != nil {
						return fmt.Errorf(`error in ID "%s" (%w)`, target, err)
					}
					if !complete {
						return fmt.Errorf(`incomplete ID "%s"`, target)
					}
					err = api.CommandForgetById(document.Id(numId))
					if err != nil {
						return err
					}
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
