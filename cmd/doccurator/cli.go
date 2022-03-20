package main

import (
	"errors"
	"flag"
	"fmt"
	"github.com/n2code/doccurator"
	cliflags "github.com/n2code/doccurator/cmd/doccurator/flags"
	cliverbs "github.com/n2code/doccurator/cmd/doccurator/verbs"
	"github.com/n2code/doccurator/internal/document"
	out "github.com/n2code/doccurator/internal/output"
	"github.com/n2code/ndocid"
	"io"
	"os"
	"path/filepath"
)

type cliRequest struct {
	verbose     bool
	quiet       bool
	thorough    bool
	noSkip      bool
	plain       bool
	action      string
	actionFlags map[string]interface{}
	actionArgs  []string
}

const defaultDbFileName = `doccurator.db`

func parseFlags(args []string, errOut io.Writer) (request *cliRequest, exitCode int) {
	flags := flag.NewFlagSet("", flag.ExitOnError)
	flags.Usage = func() {
		flags.Output().Write([]byte(`
Usage:
   doccurator [-` + cliflags.Verbose + `|-` + cliflags.Quiet + `] [-` + cliflags.Thorough + `] [-` + cliflags.All + `] [-` + cliflags.Plain + `] [-` + cliflags.Help + `] <ACTION> [FLAG] [TARGET]

 ACTIONs:  ` + cliverbs.Init + `  ` + cliverbs.Status + `  ` + cliverbs.Add + `  ` + cliverbs.Update + `  ` + cliverbs.Tidy + `  ` + cliverbs.Search + `  ` + cliverbs.Retire + `  ` + cliverbs.Forget + `  ` + cliverbs.Tree + `  ` + cliverbs.Dump + `

`))
		flags.PrintDefaults()
		flags.Output().Write([]byte(`
 FLAG(s) and TARGET(s) are action-specific.
 You can read the help on any action verb:
    doccurator <ACTION> -` + cliflags.Help + `

`))

	}

	request = &cliRequest{}
	var generalHelpRequested bool
	flags.BoolVar(&request.verbose, cliflags.Verbose, false, "Output more details on what is done (verbose mode)")
	flags.BoolVar(&request.quiet, cliflags.Quiet, false, "Output as little as possible, i.e. only requested information (quiet mode)")
	flags.BoolVar(&generalHelpRequested, cliflags.Help, false, "Display general usage help")
	flags.BoolVar(&request.thorough, cliflags.Thorough, false, "Do not apply optimizations (thorough mode), for example:\n  Unless flag is set files with unchanged modification time are not read.")
	flags.BoolVar(&request.noSkip, cliflags.All, false, "Do not skip anything during recursive scans (all mode):\n  Unless flag is set the library database file is skipped.\n  Files/folders starting with \".\" are not considered either.\n  The function of ignore files is not affected.")
	flags.BoolVar(&request.plain, cliflags.Plain, false, "Do not use terminal escape sequence features such as colors (plain mode)")

	var err error
	defer func() {
		if err != nil {
			fmt.Fprintf(errOut, "%s\nUsage help: doccurator -%s\n", err, cliflags.Help)
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
    doccurator -`+cliflags.Help+`

`)
	}

ActionParamCheck:
	switch request.action {
	case cliverbs.Status:
		argumentSpecification = " [FILEPATH...]"
		actionDescription += "Compare files in the library folder to the records.\n" +
			actionDescriptionIndent + "If one or more FILEPATHs are given only compare those files otherwise\n" +
			actionDescriptionIndent + "compare all. For an explicit list of paths all states are listed. For\n" +
			actionDescriptionIndent + "a full scan (no paths specified) unchanged tracked files are omitted."
		actionParams.Parse(request.actionArgs)
		request.actionArgs = actionParams.Args()
		//beyond flags all arguments are optional
	case cliverbs.Add, cliverbs.Update, cliverbs.Retire, cliverbs.Forget:
		argumentSpecification = " [FILEPATH...]"
		switch request.action {
		case cliverbs.Add:
			flagSpecification = " [-" + cliflags.AddAllUntracked + "] [-" + cliflags.AddWithRename + "] [-" + cliflags.AddWithForce + "] [-" + cliflags.AddButAbortOnError + "] [-" + cliflags.AddWithAutoId + " | -" + cliflags.AddWithGivenId + "=...]"
			actionDescription += "Add the file(s) at the given FILEPATH(s) to the library records.\n" +
				actionDescriptionIndent + "Interactive mode is launched if no paths are given. The user is prompted to\n" +
				actionDescriptionIndent + "decide for each untracked file whether it shall be added and/or renamed.\n" +
				actionDescriptionIndent + "Alternatively all untracked files can be added automatically via flag."
			request.actionFlags[cliflags.AddAllUntracked] = actionParams.Bool(cliflags.AddAllUntracked, false, "add all untracked files anywhere inside the library\n"+
				"(requires *standardized* filenames to extract IDs)")
			request.actionFlags[cliflags.AddWithForce] = actionParams.Bool(cliflags.AddWithForce, false, "allow adding even duplicates, moved, and obsolete files as new\n"+
				"(because this is likely undesired and thus blocked by default)")
			request.actionFlags[cliflags.AddWithAutoId] = actionParams.Bool(cliflags.AddWithAutoId, false, "automatically choose free ID based on current time if filename\n"+
				"is not *standardized* and hence ID cannot be extracted from it")
			request.actionFlags[cliflags.AddWithRename] = actionParams.Bool(cliflags.AddWithRename, false, "rename added files to standardized filename")
			request.actionFlags[cliflags.AddButAbortOnError] = actionParams.Bool(cliflags.AddButAbortOnError, false, "abort if any error occurs, do not skip issues (mass operations)")
			request.actionFlags[cliflags.AddWithGivenId] = actionParams.String(cliflags.AddWithGivenId, "", "specify new document ID instead of extracting it from filename\n"+
				"(only a single FILEPATH can be given, -"+cliflags.AddAllUntracked+" must not be used)\n"+
				"FORMAT 1: doccurator add -"+cliflags.AddWithGivenId+" 63835AEV9E my_document.pdf\n"+
				"FORMAT 2: doccurator add -"+cliflags.AddWithGivenId+"=55565IEV9E my_document.pdf")
		case cliverbs.Update:
			actionDescription += "Update the existing library records to match the current state of\n" +
				actionDescriptionIndent + "the file(s) at the given FILEPATH(s)."
		case cliverbs.Retire:
			actionDescription += "Mark the library records corresponding to the given FILEPATH(s) as\n" +
				actionDescriptionIndent + "obsolete. The real file is expected to be removed manually.\n" +
				actionDescriptionIndent + "If an identical file appears at a later point in time the library is\n" +
				actionDescriptionIndent + "thereby able to recognize it as an obsolete duplicate (\"zombie\")."
		case cliverbs.Forget:
			flagSpecification = " -" + cliflags.ForgetAllRetired + " |"
			argumentSpecification = " ID..."
			actionDescription += "Delete the library records corresponding to the given ID(s).\n" +
				actionDescriptionIndent + "Only retired documents can be forgotten."
			request.actionFlags[cliflags.ForgetAllRetired] = actionParams.Bool(cliflags.ForgetAllRetired, false, "forget all retired documents")
		}
		actionParams.Parse(request.actionArgs)
		request.actionArgs = actionParams.Args()

		verifyTargetsExist := func() {
			if actionParams.NArg() < 1 {
				err = errors.New("no targets given")
			}
		}

		switch request.action {
		case cliverbs.Add:
			if *(request.actionFlags[cliflags.AddAllUntracked].(*bool)) {
				if actionParams.NArg() != 0 {
					err = errors.New(`no FILEPATHs must be given when using flag "-` + cliflags.AddAllUntracked + `"`)
					break ActionParamCheck
				}
				if *(request.actionFlags[cliflags.AddWithGivenId].(*string)) != "" {
					err = errors.New(`flag "-` + cliflags.AddWithGivenId + `" must not be used together with "-` + cliflags.AddAllUntracked + `"`)
					break ActionParamCheck
				}
			} else if actionParams.NArg() == 0 { //interactive mode
				if actionParams.NFlag() > 0 {
					err = errors.New(`interactive mode does not take any flags`)
					break ActionParamCheck
				}
			}
			if *(request.actionFlags[cliflags.AddWithGivenId].(*string)) != "" {
				if *(request.actionFlags[cliflags.AddWithAutoId].(*bool)) {
					err = errors.New(`flag "-` + cliflags.AddWithAutoId + `" must not be used together with "-` + cliflags.AddWithGivenId + `"`)
					break ActionParamCheck
				}
				if actionParams.NArg() != 1 {
					err = errors.New(`exactly one FILEPATH must be given when using flag "-` + cliflags.AddWithGivenId + `"`)
					break ActionParamCheck
				}
			}
			if *(request.actionFlags[cliflags.AddButAbortOnError].(*bool)) {
				if actionParams.NArg() == 1 {
					err = errors.New(`"-` + cliflags.AddButAbortOnError + `" is for mass operations only`)
					break ActionParamCheck
				}
			}
		case cliverbs.Update, cliverbs.Retire:
			verifyTargetsExist()
		case cliverbs.Forget:
			if *(request.actionFlags[cliflags.ForgetAllRetired].(*bool)) {
				if actionParams.NArg() != 0 {
					err = errors.New(`no IDs must be given when using flag "-` + cliflags.ForgetAllRetired + `"`)
					break ActionParamCheck
				}
			} else {
				verifyTargetsExist()
			}
		}
	case cliverbs.Dump:
		flagSpecification = " [-" + cliflags.DumpExcludingRetired + "]"
		actionDescription += "Print all library records."
		request.actionFlags[cliflags.DumpExcludingRetired] = actionParams.Bool(cliflags.DumpExcludingRetired, false, "do not print records marked as obsolete (\"retired\")")
		actionParams.Parse(request.actionArgs)
		request.actionArgs = actionParams.Args()
		if actionParams.NArg() > 0 {
			err = errors.New("too many arguments")
			break ActionParamCheck
		}
	case cliverbs.Tree:
		flagSpecification = " [-" + cliflags.TreeWithOnlyDifferences + "] [-" + cliflags.TreeOfCurrentLocation + "]"
		actionDescription += "Display the library as a tree which represents the union of all\n" +
			actionDescriptionIndent + "library records and the files currently present in the library folder."
		request.actionFlags[cliflags.TreeWithOnlyDifferences] = actionParams.Bool(cliflags.TreeWithOnlyDifferences, false, "show only difference to library records, i.e. exclude\nunchanged tracked and tracked-as-removed files")
		request.actionFlags[cliflags.TreeOfCurrentLocation] = actionParams.Bool(cliflags.TreeOfCurrentLocation, false, "only print the subtree whose root is the current working directory")
		actionParams.Parse(request.actionArgs)
		request.actionArgs = actionParams.Args()
		if actionParams.NArg() > 0 {
			err = errors.New("command accepts no arguments, only flags")
			break ActionParamCheck
		}
	case cliverbs.Init:
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
	case cliverbs.Search:
		argumentSpecification = " ID"
		actionDescription += "Search for documents with the given ID or substring of an ID"
		actionParams.Parse(request.actionArgs)
		request.actionArgs = actionParams.Args()
		if actionParams.NArg() != 1 {
			err = errors.New("bad number of arguments, exactly one expected")
			break ActionParamCheck
		}
	case cliverbs.Tidy:
		flagSpecification = " [-" + cliflags.TidyWithoutConfirmation + "] [-" + cliflags.TidyRemovingWaste + "]"
		actionDescription += "Interactively do the needful to get the library in sync with the filesystem.\n" +
			actionDescriptionIndent + "By default only known records are considered and the filesystem is not touched."
		request.actionFlags[cliflags.TidyWithoutConfirmation] = actionParams.Bool(cliflags.TidyWithoutConfirmation, false, "suppress prompts and choose defaults (\"yes to all\")")
		request.actionFlags[cliflags.TidyRemovingWaste] = actionParams.Bool(cliflags.TidyRemovingWaste, false, "remove superfluous files with duplicate or obsolete content")
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
	if rq.noSkip {
		config.IncludeAllNamesInScan = true
	}
	if rq.plain {
		config.SuppressTerminalCodes = true
	}

	if rq.action == cliverbs.Init {
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
		case cliverbs.Dump:
			api.PrintAllRecords(*(rq.actionFlags[cliflags.DumpExcludingRetired].(*bool)))
			return nil
		case cliverbs.Tree:
			return api.PrintTree(*(rq.actionFlags[cliflags.TreeWithOnlyDifferences].(*bool)), *(rq.actionFlags[cliflags.TreeOfCurrentLocation].(*bool)))
		case cliverbs.Add:
			tryRename := *(rq.actionFlags[cliflags.AddWithRename].(*bool))
			autoId := *(rq.actionFlags[cliflags.AddWithAutoId].(*bool))
			abortOnError := *(rq.actionFlags[cliflags.AddButAbortOnError].(*bool))
			forceIfDuplicateMovedOrObsolete := *(rq.actionFlags[cliflags.AddWithForce].(*bool))
			var addedIds []document.Id
			var addErr error
			if *(rq.actionFlags[cliflags.AddAllUntracked].(*bool)) {
				addedIds, addErr = api.AddAllUntracked(forceIfDuplicateMovedOrObsolete, autoId, abortOnError)
			} else {
				if explicitId := *(rq.actionFlags[cliflags.AddWithGivenId].(*string)); explicitId != "" {
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
		case cliverbs.Update:
			for _, target := range rq.actionArgs {
				if err := api.UpdateByPath(target); err != nil {
					return err
				}
			}
			return api.PersistChanges()
		case cliverbs.Retire:
			for _, target := range rq.actionArgs {
				if err := api.RetireByPath(target); err != nil {
					return err
				}
			}
			return api.PersistChanges()
		case cliverbs.Forget:
			if *(rq.actionFlags[cliflags.ForgetAllRetired].(*bool)) {
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
		case cliverbs.Status:
			return api.PrintStatus(rq.actionArgs)
		case cliverbs.Search:
			matches := api.SearchByIdPart(rq.actionArgs[0])
			matchCount := len(matches)
			if matchCount == 0 && !rq.quiet {
				return fmt.Errorf("no matches found for ID [part]: %s", rq.actionArgs[0])
			}
			for _, match := range matches {
				fmt.Fprintf(os.Stdout, "\n%s (%s)\n", match.Path, match.StatusText)
				api.PrintRecord(match.Id)
			}
			fmt.Fprintf(os.Stdout, "\n\n%d %s found\n", matchCount, out.Plural(matchCount, "match", "matches"))
			return nil
		case cliverbs.Tidy:
			choice := PromptUser(!rq.plain)
			if *(rq.actionFlags[cliflags.TidyWithoutConfirmation].(*bool)) {
				choice = AutoChooseDefaultOption(rq.quiet)
			} else {
				fmt.Fprint(os.Stdout, "(To abort and undo everything: SIGINT/Ctrl+C during prompts)\n")
			}
			removeWaste := *(rq.actionFlags[cliflags.TidyRemovingWaste].(*bool))
			decisionsMade, foundWaste, cancelled := api.InteractiveTidy(choice, removeWaste)
			if cancelled {
				return fmt.Errorf("operation aborted, undo requested")
			}
			if decisionsMade == 0 && !rq.quiet {
				fmt.Fprint(os.Stdout, "Nothing to do!\n")
				if foundWaste && !removeWaste {
					fmt.Fprint(os.Stdout, "(Duplicate or obsolete files exist. Repeat with flag -"+cliflags.TidyRemovingWaste+" to remove.)\n")
				}
				fmt.Fprint(os.Stdout, "\n")
			}
			return api.PersistChanges()
		default:
			panic("bad action")
		}
	}
}

func main() {
	rq, rc := parseFlags(os.Args[1:], os.Stderr)
	if rc != 0 || rq == nil {
		os.Exit(rc)
	}
	if err := rq.execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		switch rq.action {
		case cliverbs.Add, cliverbs.Update, cliverbs.Tidy:
			fmt.Fprintln(os.Stderr, "(library not modified because of errors)")
		}
		os.Exit(1)
	}
	os.Exit(0)
}
