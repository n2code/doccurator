package doccinator

import (
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path"

	document "github.com/n2code/doccinator/internal/document"
	. "github.com/n2code/doccinator/internal/library"
)

const libraryPointerFileName = ".doccinator"

var appLib LibraryApi
var libFile string

type CommandError struct {
	message string
	cause   error
}

func (e *CommandError) Error() string {
	return fmt.Sprintf("%s: %s", e.message, e.cause)
}

func (e *CommandError) Unwrap() error {
	return e.cause
}

func newCommandError(message string, cause error) *CommandError {
	return &CommandError{message: message, cause: cause}
}

// Records a new document in the library
func CommandAdd(id document.DocumentId, fileAbsolutePath string) error {
	doc, err := appLib.CreateDocument(id)
	if err != nil {
		return newCommandError("document creation failed", err)
	}
	appLib.SetDocumentPath(doc, fileAbsolutePath)
	appLib.UpdateDocumentFromFile(doc)
	return nil
}

// Updates an existing document in the library
func CommandUpdateByPath(fileAbsolutePath string) error {
	doc, exists := appLib.GetDocumentByPath(fileAbsolutePath)
	if !exists {
		return newCommandError(fmt.Sprintf("path unknown: %s", fileAbsolutePath), nil)
	}
	appLib.UpdateDocumentFromFile(doc)
	return nil
}

// Removes an existing document from the library
func CommandRemoveByPath(fileAbsolutePath string) error {
	doc, exists := appLib.GetDocumentByPath(fileAbsolutePath)
	if !exists {
		return newCommandError(fmt.Sprintf("path not on record: %s", fileAbsolutePath), nil)
	}
	appLib.ForgetDocument(doc)
	return nil
}

// Outputs all library records
func CommandDump(out io.Writer) {
	allRecords := appLib.AllRecordsAsText()

	fmt.Fprint(out, allRecords)
	if len(allRecords) == 0 {
		fmt.Fprintln(out, "<no records>")
	}
}

// Calculates states for all present and recorded paths.
//  Tracked and removed paths require special flag triggers to be listed.
func CommandScan(out io.Writer) error {
	appLib.ChdirToRoot()
	workingDir, _ := os.Getwd()
	fmt.Fprintf(out, "Scanning library in %s ...\n", workingDir)
	paths := appLib.Scan()
	for _, checkedPath := range paths {
		fmt.Fprintf(out, "[%s] %s\n", string(checkedPath.Status()), checkedPath.PathRelativeToLibraryRoot())
	}
	return nil
}

// Auto pilot adds untracked paths, updates touched & moved paths, and removes duplicates.
//  Modified and missing are not changed but reported.
//  If additional flags are passed modified paths are updated and/or missing paths removed.
//  Unknown paths are reported.
func CommandAuto() error {
	return nil
}

func InitAppLibrary(absoluteRoot string) {
	appLib = MakeRuntimeLibrary()
	appLib.SetRoot(absoluteRoot)
}

func DiscoverAppLibrary(startingDirectoryAbsolute string) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("library not found: %w", err)
		}
	}()
	currentDir := startingDirectoryAbsolute
	for {
		pointerFile := path.Join(currentDir, libraryPointerFileName)
		stat, statErr := os.Stat(pointerFile)
		if statErr == nil && stat.Mode().IsRegular() {
			contents, readErr := os.ReadFile(pointerFile)
			if readErr != nil {
				return readErr
			}
			url, parseErr := url.Parse(string(contents))
			if parseErr != nil {
				return parseErr
			}
			if url.Scheme != "file" {
				return errors.New("scheme of URL in library locator file missing or unsupported: " + url.Scheme)
			}
			libFile = url.Path
			appLib = MakeRuntimeLibrary()
			appLib.LoadFromLocalFile(libFile)
			return nil
		} else if errors.Is(statErr, os.ErrNotExist) {
			if currentDir == "/" {
				return errors.New("stopping at filesystem root")
			}
			currentDir = path.Dir(currentDir)
		} else {
			return statErr
		}
	}
}

func CreateDatabase(absolutePath string) {
	libFile = absolutePath
	appLib.SaveToLocalFile(libFile, false)
}

func PersistDatabase() {
	appLib.SaveToLocalFile(libFile, true)
}
