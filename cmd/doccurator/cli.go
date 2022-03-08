package main

import (
	"errors"
	"flag"
	"fmt"
	"github.com/n2code/doccurator"
	"github.com/n2code/doccurator/internal/document"
	"github.com/n2code/doccurator/internal/output"
	"github.com/n2code/ndocid"
	"io"
	"os"
	"path/filepath"
)

type cliRequest struct {
	verbose     bool
	quiet       bool
	thorough    bool
	plain       bool
	action      string
	actionFlags map[string]interface{}
	actionArgs  []string
}

const defaultDbFileName = `doccurator.db`

func parseFlags(args []string, out io.Writer, errOut io.Writer) (request *cliRequest, exitCode int) {
	flags := flag.NewFlagSet("", flag.ExitOnError)
	flags.Usage = func() {
		flags.Output().Write([]byte(`
Usage:
   doccurator [-v|-q] [-t] [-p] [-h] <ACTION> [FLAG] [TARGET]

 ACTIONs:  init  status  add  update  tidy  search  retire  forget  tree  dump

`))
		flags.PrintDefaults()
		flags.Output().Write([]byte(`
 FLAG(s) and TARGET(s) are action-specific.
 You can read the help on any action:
    doccurator <ACTION> -h

`))

	}

	request = &cliRequest{}
	var generalHelpRequested bool
	flags.BoolVar(&request.verbose, "v", false, "Output more details on what is done (verbose mode)")
	flags.BoolVar(&request.quiet, "q", false, "Output as little as possible, i.e. only requested information (quiet mode)")
	flags.BoolVar(&generalHelpRequested, "h", false, "Display general usage help")
	flags.BoolVar(&request.thorough, "t", false, "Do not apply optimizations (thorough mode), for example:\n  Unless flag is set files whose modification time is unchanged are not read.")
	flags.BoolVar(&request.plain, "p", false, "Do not use terminal escape sequence features such as colors (plain mode)")

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
		err = errors.New("no arguments given")
		return
	}
	if request.verbose && request.quiet {
		err = errors.New("quiet mode and verbose mode are mutually exclusive")
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

ActionParamCheck:
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
			flagSpecification = " [-all-untracked] [-rename] [-force] [-abort-on-error] [-auto-id | -id=...]"
			actionDescription += "Add the file(s) at the given FILEPATH(s) to the library records.\n" +
				actionDescriptionIndent + "Interactive mode is launched if no paths are given. The user is prompted to\n" +
				actionDescriptionIndent + "decide for each untracked file whether it shall be added and/or renamed.\n" +
				actionDescriptionIndent + "Alternatively all untracked files can be added automatically via flag."
			request.actionFlags["all-untracked"] = actionParams.Bool("all-untracked", false, "add all untracked files anywhere inside the library\n"+
				"(requires *standardized* filenames to extract IDs)")
			request.actionFlags["force"] = actionParams.Bool("force", false, "allow adding even duplicates, moved, and obsolete files as new\n"+
				"(because this is likely undesired and thus blocked by default)")
			request.actionFlags["auto-id"] = actionParams.Bool("auto-id", false, "automatically choose free ID based on current time if filename\n"+
				"is not *standardized* and hence ID cannot be extracted from it")
			request.actionFlags["rename"] = actionParams.Bool("rename", false, "rename added files to standardized filename")
			request.actionFlags["abort-on-error"] = actionParams.Bool("abort-on-error", false, "abort if any error occurs, do not skip issues (mass operations)")
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

		verifyTargetsExist := func() {
			if actionParams.NArg() < 1 {
				err = errors.New("no targets given")
			}
		}

		switch request.action {
		case "add":
			if *(request.actionFlags["all-untracked"].(*bool)) {
				if actionParams.NArg() != 0 {
					err = errors.New(`no FILEPATHs must be given when using flag "-all-untracked"`)
					break ActionParamCheck
				}
				if *(request.actionFlags["id"].(*string)) != "" {
					err = errors.New(`flag "-id" must not be used together with "-all-untracked"`)
					break ActionParamCheck
				}
			} else if actionParams.NArg() == 0 { //interactive mode
				if actionParams.NFlag() > 0 {
					err = errors.New(`interactive mode does not take any flags`)
					break ActionParamCheck
				}
			}
			if *(request.actionFlags["id"].(*string)) != "" {
				if *(request.actionFlags["auto-id"].(*bool)) {
					err = errors.New(`flag "-auto-id" must not be used together with "-id"`)
					break ActionParamCheck
				}
				if actionParams.NArg() != 1 {
					err = errors.New(`exactly one FILEPATH must be given when using flag "-id"`)
					break ActionParamCheck
				}
			}
			if *(request.actionFlags["abort-on-error"].(*bool)) {
				if actionParams.NArg() == 1 {
					err = errors.New(`"-abort-on-error" is for mass operations only`)
					break ActionParamCheck
				}
			}
		case "update", "retire":
			verifyTargetsExist()
		case "forget":
			if *(request.actionFlags["all-retired"].(*bool)) {
				if actionParams.NArg() != 0 {
					err = errors.New(`no IDs must be given when using flag "-all-retired"`)
					break ActionParamCheck
				}
			} else {
				verifyTargetsExist()
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
			break ActionParamCheck
		}
	case "tree":
		flagSpecification = " [-diff] [-here]"
		actionDescription += "Display the library as a tree which represents the union of all\n" +
			actionDescriptionIndent + "library records and the files currently present in the library folder."
		request.actionFlags["diff"] = actionParams.Bool("diff", false, "show only difference to library records, i.e. exclude\nunchanged tracked and tracked-as-removed files")
		request.actionFlags["here"] = actionParams.Bool("here", false, "only print the subtree whose root is the current working directory")
		actionParams.Parse(request.actionArgs)
		request.actionArgs = actionParams.Args()
		if actionParams.NArg() > 0 {
			err = errors.New("command accepts no arguments, only flags")
			break ActionParamCheck
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
			break ActionParamCheck
		}
	case "search":
		argumentSpecification = " ID"
		actionDescription += "Search for documents with the given ID or substring of an ID"
		actionParams.Parse(request.actionArgs)
		request.actionArgs = actionParams.Args()
		if actionParams.NArg() != 1 {
			err = errors.New("bad number of arguments, exactly one expected")
			break ActionParamCheck
		}
	case "tidy":
		flagSpecification = " [-no-confirm] [-remove-waste-files]"
		actionDescription += "Interactively do the needful to get the library in sync with the filesystem.\n" +
			actionDescriptionIndent + "By default only known records are considered and the filesystem is not touched."
		request.actionFlags["no-confirm"] = actionParams.Bool("no-confirm", false, "suppress prompts and choose defaults (\"yes to all\")")
		request.actionFlags["remove-waste-files"] = actionParams.Bool("remove-waste-files", false, "remove superfluous files with duplicate or obsolete content")
		actionParams.Parse(request.actionArgs)
		request.actionArgs = actionParams.Args()
		if actionParams.NArg() > 0 {
			err = errors.New("command accepts no arguments, only flags")
			break ActionParamCheck
		}
	default:
		err = fmt.Errorf(`unknown action "%s"`, request.action)
	}
	return
}

func (rq *cliRequest) execute() (execErr error) {
	var config doccurator.HandleConfig
	if rq.verbose {
		config.Verbosity = doccurator.VerboseMode
	}
	if rq.quiet {
		config.Verbosity = doccurator.QuietMode
	}
	if rq.thorough {
		config.Optimization = doccurator.ThoroughMode
	}
	if rq.plain {
		config.SuppressTerminalCodes = true
	}

	if rq.action == "init" {
		_, err := doccurator.New(rq.actionArgs[0], filepath.Join(rq.actionArgs[0], defaultDbFileName), config)
		return err
	} else {
		workingDir, _ := os.Getwd()
		api, err := doccurator.Open(workingDir, config)
		if err != nil {
			return err
		}
		defer func() {
			if execErr != nil {
				api.RollbackAllFilesystemChanges()
			}
		}()

		switch rq.action {
		case "dump":
			api.PrintAllRecords(*(rq.actionFlags["exclude-retired"].(*bool)))
			return nil
		case "tree":
			return api.PrintTree(*(rq.actionFlags["diff"].(*bool)), *(rq.actionFlags["here"].(*bool)))
		case "add":
			tryRename := *(rq.actionFlags["rename"].(*bool))
			autoId := *(rq.actionFlags["auto-id"].(*bool))
			abortOnError := *(rq.actionFlags["abort-on-error"].(*bool))
			forceIfDuplicateMovedOrObsolete := *(rq.actionFlags["force"].(*bool))
			var addedIds []document.Id
			var addErr error
			if *(rq.actionFlags["all-untracked"].(*bool)) {
				addedIds, addErr = api.AddAllUntracked(forceIfDuplicateMovedOrObsolete, autoId, abortOnError)
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
					addErr = api.Add(newId, rq.actionArgs[0], forceIfDuplicateMovedOrObsolete)
					if addErr == nil {
						addedIds = append(addedIds, newId)
					}
				} else {
					if len(rq.actionArgs) == 0 {
						fmt.Fprint(os.Stdout, "(To stop adding more files: SIGINT/Ctrl+C during prompts)\n")
						cancelled := api.InteractiveAdd(PromptUser(!rq.plain))
						if cancelled {
							fmt.Fprint(os.Stdout, "(Interactive mode interrupted, repeat command to continue)\n")
						}
					} else {
						addedIds, addErr = api.AddMultiple(rq.actionArgs, forceIfDuplicateMovedOrObsolete, autoId, abortOnError)
					}
				}
			}
			if addErr != nil {
				return addErr
			}
			if tryRename {
				for _, addedId := range addedIds {
					err := api.StandardizeLocation(addedId)
					if err != nil {
						return fmt.Errorf(`renaming file of document %s failed: %w`, addedId, err)
					}
				}
			}
			return api.PersistChanges()
		case "update":
			for _, target := range rq.actionArgs {
				if err := api.UpdateByPath(target); err != nil {
					return err
				}
			}
			return api.PersistChanges()
		case "retire":
			for _, target := range rq.actionArgs {
				if err := api.RetireByPath(target); err != nil {
					return err
				}
			}
			return api.PersistChanges()
		case "forget":
			if *(rq.actionFlags["all-retired"].(*bool)) {
				api.ForgetAllObsolete()
			} else {
				for _, target := range rq.actionArgs {
					numId, err, complete := ndocid.Decode(target)
					if err != nil {
						return fmt.Errorf(`error in ID "%s" (%w)`, target, err)
					}
					if !complete {
						return fmt.Errorf(`incomplete ID "%s"`, target)
					}
					if err := api.ForgetById(document.Id(numId), false); err != nil {
						return err
					}
				}
			}
			return api.PersistChanges()
		case "status":
			return api.PrintStatus(rq.actionArgs)
		case "search":
			matches := api.SearchByIdPart(rq.actionArgs[0])
			matchCount := len(matches)
			if matchCount == 0 && !rq.quiet {
				return fmt.Errorf("no matches found for ID [part]: %s", rq.actionArgs[0])
			}
			for _, match := range matches {
				fmt.Fprintf(os.Stdout, "\n%s (%s)\n", match.Path, match.StatusText)
				api.PrintRecord(match.Id)
			}
			fmt.Fprintf(os.Stdout, "\n\n%d %s found\n", matchCount, output.Plural(matchCount, "match", "matches"))
			return nil
		case "tidy":
			choice := PromptUser(!rq.plain)
			if *(rq.actionFlags["no-confirm"].(*bool)) {
				choice = AutoChooseDefaultOption(rq.quiet)
			} else {
				fmt.Fprint(os.Stdout, "(To abort and undo everything: SIGINT/Ctrl+C during prompts)\n")
			}
			cancelled := api.InteractiveTidy(choice, *(rq.actionFlags["remove-waste-files"].(*bool)))
			if cancelled {
				return fmt.Errorf("operation aborted, undo requested")
			}
			return api.PersistChanges()
		default:
			panic("bad action")
		}
	}
}

func main() {
	rq, rc := parseFlags(os.Args[1:], os.Stdout, os.Stderr)
	if rc != 0 || rq == nil {
		os.Exit(rc)
	}
	if err := rq.execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		switch rq.action {
		case "add", "update", "tidy":
			fmt.Fprintln(os.Stderr, "(library not modified because of errors)")
		}
		os.Exit(1)
	}
	os.Exit(0)
}
