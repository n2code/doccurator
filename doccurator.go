package doccurator

import (
	"fmt"
	"github.com/n2code/ndocid"
	"io"
	"os"
	"path/filepath"
	"regexp"

	"github.com/n2code/doccurator/internal/document"
	"github.com/n2code/doccurator/internal/library"
)

type VerbosityLevel int

const (
	Default VerbosityLevel = iota
	VerboseMode
	QuietMode
)

// zero value is a sensible default
type CreateConfig struct {
	Verbosity VerbosityLevel
}

type Doccurator interface {
	CommandAddSingle(id document.Id, path string, allowForDuplicateMovedAndObsolete bool) error
	CommandAddAllUntracked(allowDuplicates bool) error
	CommandStandardizeLocation(id document.Id) error
	CommandUpdateByPath(path string) error
	CommandRetireByPath(path string) error
	CommandForgetById(id document.Id) error
	CommandForgetAllObsolete()
	CommandDump(excludeRetired bool)
	CommandTree(excludeUnchanged bool) error
	CommandStatus(paths []string) error
	CommandAuto() error
	PersistChanges() error
}

type doccurator struct {
	appLib     library.Api
	libFile    string    //absolute, system-native path
	out        io.Writer //essential output (i.e. requested information)
	extraOut   io.Writer //more output for convenience (repeats context)
	verboseOut io.Writer //most output, talkative
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
	return nil

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

func makeDoccurator(config CreateConfig) (docc *doccurator) {
	docc = &doccurator{out: os.Stdout, extraOut: io.Discard, verboseOut: io.Discard}
	switch config.Verbosity {
	case VerboseMode:
		docc.verboseOut = os.Stdout
		fallthrough
	case Default:
		docc.extraOut = os.Stdout
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
