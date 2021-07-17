package doccinator

import (
	"errors"
	"fmt"
	"io/fs"
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
		panic("creation failed")
	}
	appLib.SetDocumentPath(doc, fileAbsolutePath)
	doc.UpdateFromFile()
}

// update an existing document in the library
func CommandUpdateByPath(fileAbsolutePath string) error {
	doc, exists := appLib.GetDocumentByPath(fileAbsolutePath)
	if !exists {
		return errors.New(fmt.Sprint("path unknown: ", fileAbsolutePath))
	}
	doc.UpdateFromFile()
	return nil
}

// remove an existing document from the library
func CommandRemoveByPath(fileAbsolutePath string) error {
	doc, exists := appLib.GetDocumentByPath(fileAbsolutePath)
	if !exists {
		return errors.New(fmt.Sprint("path not on record: ", fileAbsolutePath))
	}
	return appLib.RemoveDocument(doc)
}

func CommandList() {
	fmt.Print(appLib.AllRecordsAsText())
}

func CommandStatus() error {
	files, err := appLib.Scan()
	if err != nil {
		return errors.New(fmt.Sprint("scanning failed: ", err))
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

func DiscoverAppLibrary() bool {
	currentDir := getRealWorkingDirectory()
	for {
		pointerFile := path.Join(currentDir, libraryPointerFileName)
		stat, err := os.Stat(pointerFile)
		if err == nil && stat.Mode().IsRegular() {
			contents, err := os.ReadFile(pointerFile)
			if err != nil {
				panic(err)
			}
			url, err := url.Parse(string(contents))
			if err != nil {
				panic(err)
			}
			if url.Scheme != "file" {
				panic(errors.New("scheme of URL in library locator file missing or unsupported: " + url.Scheme))
			}
			appLib = MakeRuntimeLibrary()
			appLib.LoadFromLocalFile(url.Path)
			return true
		} else if errors.Is(err, fs.ErrNotExist) {
			if currentDir == "/" {
				return false
			}
			currentDir = path.Dir(currentDir)
		} else {
			panic(err)
		}
	}
}

func PersistDatabase() {
	appLib.SaveToLocalFile("/tmp/doccinator.db")
}
