package doccinator

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path"

	. "github.com/n2code/doccinator/internal"
)

const libraryPointerFileName = ".doccinator"

var appLib Library

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

// add records a new document in the library
func CommandAdd(id DocumentId, fileAbsolutePath string) error {
	doc, err := appLib.CreateDocument(id)
	if err != nil {
		return newCommandError("document creation failed", err)
	}
	appLib.SetDocumentPath(doc, fileAbsolutePath)
	appLib.UpdateDocumentFromFile(doc)
	return nil
}

// update an existing document in the library
func CommandUpdateByPath(fileAbsolutePath string) error {
	doc, exists := appLib.GetDocumentByPath(fileAbsolutePath)
	if !exists {
		return newCommandError(fmt.Sprintf("path unknown: %s", fileAbsolutePath), nil)
	}
	appLib.UpdateDocumentFromFile(doc)
	return nil
}

// remove an existing document from the library
func CommandRemoveByPath(fileAbsolutePath string) error {
	doc, exists := appLib.GetDocumentByPath(fileAbsolutePath)
	if !exists {
		return newCommandError(fmt.Sprintf("path not on record: %s", fileAbsolutePath), nil)
	}
	appLib.ForgetDocument(doc)
	return nil
}

func CommandList() {
	allRecords := appLib.AllRecordsAsText()
	fmt.Print(allRecords)
	if len(allRecords) == 0 {
		fmt.Println("<no records>")
	}
}

func CommandStatus() error {
	files, err := appLib.Scan()
	if err != nil {
		return newCommandError("scanning failed", err)
	}
	workingDirectory, err := os.Getwd()
	if err != nil {
		return newCommandError("working directory indeterminable", err)
	}
	files.DisplayDelta(workingDirectory)
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
			appLib = MakeRuntimeLibrary()
			appLib.LoadFromLocalFile(url.Path)
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
	appLib.SaveToLocalFile(absolutePath, false)
}

func PersistDatabase(absolutePath string) {
	appLib.SaveToLocalFile(absolutePath, true)
}
