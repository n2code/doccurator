package doccurator

import (
	"fmt"
	"github.com/n2code/doccurator/internal"
	"github.com/n2code/doccurator/internal/document"
	"github.com/n2code/doccurator/internal/library"
	"github.com/n2code/ndocid"
	"io"
	"os"
	"path/filepath"
	"regexp"
)

type VerbosityLevel int

const (
	DefaultVerbosity VerbosityLevel = iota
	VerboseMode
	QuietMode
)

type OptimizationLevel int

const (
	DefaultOptimizations OptimizationLevel = iota
	ThoroughMode
)

// zero value is a sensible default
type CreateConfig struct {
	Verbosity    VerbosityLevel
	Optimization OptimizationLevel
}

type SearchResult struct {
	Id           document.Id
	RelativePath string
	StatusText   string
}

// RequestChoice represents a single-choice decision callback, the first option is considered the default "yes"-like choice.
// If cleanup is set the implementation is recommended to remove the choice presentation after selection.
type RequestChoice func(request string, options []string, cleanup bool) (choice string)

const ChoiceAborted = ""

type Doccurator interface {
	GetFreeId() document.Id
	Add(id document.Id, path string, allowForDuplicateMovedAndObsolete bool) error
	AddMultiple(paths []string, allowForDuplicateMovedAndObsolete bool, generateMissingIds bool, abortOnError bool) (added []document.Id, err error)
	AddAllUntracked(allowForDuplicateMovedAndObsolete bool, generateMissingIds bool, abortOnError bool) (added []document.Id, err error)
	StandardizeLocation(id document.Id) error
	UpdateByPath(path string) error
	SearchByIdPart(part string) []SearchResult
	RetireByPath(path string) error
	ForgetById(id document.Id, forceRetire bool) error
	ForgetAllObsolete()
	PrintRecord(id document.Id)
	PrintAllRecords(excludeRetired bool)
	PrintTree(excludeUnchanged bool) error
	PrintStatus(paths []string) error
	InteractiveTidy(choice RequestChoice, removeWaste bool) error
	PersistChanges() error
	RollbackAllFilesystemChanges() error
}

type doccurator struct {
	appLib            library.Api
	rollbackLog       []func() error
	libFile           string    //absolute, system-native path
	out               io.Writer //essential output (i.e. requested information)
	extraOut          io.Writer //more output for convenience (repeats context)
	verboseOut        io.Writer //most output, talkative
	errOut            io.Writer //error output
	optimizedFsAccess bool
}

//represents file.23456X777.ndoc.ext or file_without_ext.23456X777.ndoc or .23456X777.ndoc.ext_only
var ndocFileNameRegex = regexp.MustCompile(`^.*\.(` + document.IdPattern + `)\.ndoc(?:\.[^.]*)?$`)

func New(root string, database string, config CreateConfig) (Doccurator, error) {
	handle := makeDoccurator(config)
	err := handle.createLibrary(mustAbsFilepath(root), mustAbsFilepath(database))
	if err != nil {
		return nil, fmt.Errorf("library create error: %w", err)
	}
	return handle, nil
}

func Open(directory string, config CreateConfig) (Doccurator, error) {
	handle := makeDoccurator(config)
	err := handle.loadLibrary(mustAbsFilepath(directory))
	if err != nil {
		return nil, fmt.Errorf("library load error: %w", err)
	}
	return handle, nil
}

func (d *doccurator) PersistChanges() error {
	if err := d.appLib.SaveToLocalFile(d.libFile, true); err != nil {
		return fmt.Errorf("library save error: %w", err)
	}
	d.rollbackLog = nil
	return nil

}

func (d *doccurator) RollbackAllFilesystemChanges() error {
	for i := len(d.rollbackLog) - 1; i >= 0; i-- {
		rollbackStep := d.rollbackLog[i]
		err := rollbackStep()
		if err != nil {
			d.rollbackLog = d.rollbackLog[:i+1] //drop all after failing
			return fmt.Errorf("filesystem rollback incomplete: %w", err)
		}
	}
	if len(d.rollbackLog) > 0 { //do not print if rollback is no-op
		fmt.Fprintln(d.extraOut, "Executed filesystem rollback")
	}
	d.rollbackLog = nil
	return nil
}

func (d *doccurator) GetFreeId() document.Id {
	candidate := internal.UnixTimestampNow()
	for candidate > 0 {
		id := document.Id(candidate)
		if _, exists := d.appLib.GetDocumentById(id); !exists {
			return id
		}
		candidate--
	}
	return document.MissingId
}

func ExtractIdFromStandardizedFilename(path string) (document.Id, error) {
	filename := filepath.Base(path)
	matches := ndocFileNameRegex.FindStringSubmatch(filename)
	if matches == nil {
		return 0, fmt.Errorf("ID missing in filename %s (expected format <name>.<ID>.ndoc.<ext>, e.g. notes.txt.23352M4R96Z.ndoc.txt)", filename)
	}
	textId := matches[1]
	numId, err, _ := ndocid.Decode(textId)
	if err != nil {
		return 0, fmt.Errorf(`bad ID in filename %s (%w)`, filename, err)
	}
	return document.Id(numId), nil
}

func makeDoccurator(config CreateConfig) (instance *doccurator) {
	instance = &doccurator{out: os.Stdout, extraOut: io.Discard, verboseOut: io.Discard, errOut: os.Stderr}
	switch config.Verbosity {
	case VerboseMode:
		instance.verboseOut = os.Stdout
		fallthrough
	case DefaultVerbosity:
		instance.extraOut = os.Stdout
	}
	if config.Optimization == DefaultOptimizations {
		instance.optimizedFsAccess = true
	}
	return
}

func mustAbsFilepath(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		panic(err)
	}
	return abs
}

func mustRelFilepathToWorkingDir(path string) string {
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	rel, err := filepath.Rel(wd, path)
	if err != nil {
		panic(err)
	}
	return rel
}
