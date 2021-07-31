package doccinator

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"

	. "github.com/n2code/doccinator/internal"
)

const libraryPointerFileName = ".doccinator"

var appLib Library

// add records a new document in the library
func CommandAdd(id DocumentId, fileAbsolutePath string) {
	doc, err := appLib.CreateDocument(id)
	if err != nil {
		panic(fmt.Errorf("document creation failed: %w", err))
	}
	appLib.SetDocumentPath(doc, fileAbsolutePath)
	doc.UpdateFromFile()
}

// update an existing document in the library
func CommandUpdateByPath(fileAbsolutePath string) error {
	doc, exists := appLib.GetDocumentByPath(fileAbsolutePath)
	if !exists {
		return fmt.Errorf("path unknown: %s", fileAbsolutePath)
	}
	doc.UpdateFromFile()
	return nil
}

// remove an existing document from the library
func CommandRemoveByPath(fileAbsolutePath string) error {
	doc, exists := appLib.GetDocumentByPath(fileAbsolutePath)
	if !exists {
		return fmt.Errorf("path not on record: %s", fileAbsolutePath)
	}
	return appLib.RemoveDocument(doc)
}

func CommandList() {
	fmt.Print(appLib.AllRecordsAsText())
}

func CommandStatus() error {
	files, err := appLib.Scan()
	if err != nil {
		return fmt.Errorf("scanning failed: %w", err)
	}
	workingDirectory, err := os.Getwd()
	if err != nil {
		return err
	}
	files.DisplayDelta(workingDirectory)
	return nil
}

func getRealWorkingDirectory() string {
	workingDirectory, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	absoluteWorkingDirectory, err := filepath.Abs(workingDirectory)
	if err != nil {
		panic(err)
	}
	realWorkingDirectory, err := filepath.EvalSymlinks(absoluteWorkingDirectory)
	if err != nil {
		panic(err)
	}
	return realWorkingDirectory
}

func InitAppLibrary() {
	appLib = MakeRuntimeLibrary()
	appLib.SetRoot(getRealWorkingDirectory())
}

func DiscoverAppLibrary() (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("library not found: %w", err)
		}
	}()
	currentDir := getRealWorkingDirectory()
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

func PersistDatabase() {
	appLib.SaveToLocalFile("/tmp/doccinator.db")
}
