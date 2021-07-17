package doccinator

import (
	"errors"
	"fmt"
	"os"

	. "github.com/n2code/doccinator/internal"
)

var appLib = MakeLibrary("/tmp")

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

func PersistDatabase() {
	appLib.SaveToFile("/tmp/doccinator.db")
}
