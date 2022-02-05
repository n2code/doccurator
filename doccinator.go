package doccinator

import (
	"fmt"
	"github.com/n2code/ndocid"
	"io"
	"os"
	"path/filepath"
	"regexp"

	"github.com/n2code/doccinator/internal/document"
	. "github.com/n2code/doccinator/internal/library"
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

type Doccinator interface {
	CommandAddSingle(id document.DocumentId, path string) error
	CommandAddAllUntracked() error
	CommandStandardizeLocation(id document.DocumentId) error
	CommandUpdateByPath(path string) error
	CommandRetireByPath(path string) error
	CommandForgetById(id document.DocumentId) error
	CommandForgetAllObsolete()
	CommandDump(excludeRetired bool)
	CommandTree(excludeUnchanged bool) error
	CommandStatus(paths []string) error
	CommandAuto() error
	PersistChanges() error
}

type doccinator struct {
	appLib     LibraryApi
	libFile    string    //absolute, system-native path
	out        io.Writer //essential output (i.e. requested information)
	extraOut   io.Writer //more output for convenience (repeats context)
	verboseOut io.Writer //most output, talkative
}

//represents file.23456X777.ndoc.ext or file_without_ext.23456X777.ndoc or .23456X777.ndoc.ext_only
var ndocFileNameRegex = regexp.MustCompile(`^.*\.(` + document.IdPattern + `)\.ndoc(?:\.[^.]*)?$`)

func New(root string, database string, config CreateConfig) (Doccinator, error) {
	handle := makeDoccinator(config)
	err := handle.createLibrary(mustAbsFilepath(root), mustAbsFilepath(database))
	if err != nil {
		return nil, fmt.Errorf("library create error: %w", err)
	}
	return handle, nil
}

func Open(directory string, config CreateConfig) (Doccinator, error) {
	handle := makeDoccinator(config)
	err := handle.loadLibrary(mustAbsFilepath(directory))
	if err != nil {
		return nil, fmt.Errorf("library load error: %w", err)
	}
	return handle, nil
}

func (d *doccinator) PersistChanges() error {
	if err := d.appLib.SaveToLocalFile(d.libFile, true); err != nil {
		return fmt.Errorf("library save error: %w", err)
	}
	return nil

}

func ExtractIdFromStandardizedFilename(path string) (document.DocumentId, error) {
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
	return document.DocumentId(numId), nil
}

func makeDoccinator(config CreateConfig) (docc *doccinator) {
	docc = &doccinator{out: os.Stdout, extraOut: io.Discard, verboseOut: io.Discard}
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
