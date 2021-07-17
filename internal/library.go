package internal

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type fileStatus rune

const (
	newFile        fileStatus = '+'
	modifiedFile   fileStatus = '!'
	movedFile      fileStatus = '>'
	unmodifiedFile fileStatus = '='
	purgedFile     fileStatus = 'X'
	missingFile    fileStatus = '?'
)

type LibraryFile struct {
	libraryPath string
	status      fileStatus
}

type LibraryFiles []LibraryFile

// The Library API expects absolute system-native paths (with respect to the directory separator)
type Library interface {
	CreateDocument(DocumentId) (*Document, error)
	SetDocumentPath(doc *Document, absolutePath string)
	GetDocumentByPath(string) (doc *Document, exists bool)
	RemoveDocument(*Document) error
	Scan() (LibraryFiles, error)
	SaveToFile(path string)
	LoadFromFile(path string)
	ChdirToRoot()
}

func MakeLibrary(absoluteRoot string) Library {
	return &library{
		rootPath:     absoluteRoot,
		documents:    make(map[DocumentId]*Document),
		relPathIndex: make(map[string]*Document)}
}

func (lib *library) SaveToFile(path string) {
	jsonBlob, err := json.Marshal(lib.documents)
	if err != nil {
		panic(err)
	}
	err = os.WriteFile(path, jsonBlob, fs.ModePerm)
	if err != nil {
		panic(err)
	}
}

func (lib *library) LoadFromFile(path string) {
	jsonBlob, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}
	lib.documents = make(map[DocumentId]*Document)
	err = json.Unmarshal(jsonBlob, &lib.documents)
	if err != nil {
		panic(err)
	}
	lib.relPathIndex = make(map[string]*Document)
	for _, doc := range lib.documents {
		lib.relPathIndex[doc.localStorage.pathRelativeToLibrary()] = doc
	}
}

func (lib *library) CreateDocument(id DocumentId) (doc *Document, err error) {
	if _, exists := lib.documents[id]; exists {
		err = errors.New("document ID already exists")
		return
	}
	doc = &Document{
		id:       id,
		recorded: unixTimestamp(time.Now().Unix()),
	}
	lib.documents[id] = doc
	return
}

func (lib *library) SetDocumentPath(doc *Document, absolutePath string) {
	newRelativePath, inLibrary := lib.getPathRelativeToLibraryRoot((absolutePath))
	if !inLibrary {
		panic("path not inside library")
	}
	delete(lib.relPathIndex, doc.localStorage.pathRelativeToLibrary())
	doc.localStorage.setFromRelativePath(newRelativePath)
	lib.relPathIndex[newRelativePath] = doc
}

func (lib *library) GetDocumentByPath(absolutePath string) (doc *Document, exists bool) {
	relativePath, inLibrary := lib.getPathRelativeToLibraryRoot((absolutePath))
	if !inLibrary {
		exists = false
		return
	}
	doc, exists = lib.relPathIndex[relativePath]
	return
}

func (lib *library) RemoveDocument(doc *Document) error {
	relativePath := doc.localStorage.pathRelativeToLibrary()
	doc, exists := lib.relPathIndex[relativePath]
	if !exists {
		return errors.New(fmt.Sprint("document unknown: ", relativePath))
	}
	delete(lib.relPathIndex, relativePath)
	delete(lib.documents, doc.id)
	return nil
}

func (lib *library) Scan() (libraryFiles LibraryFiles, err error) {
	libraryFiles = make(LibraryFiles, 0, len(lib.documents))
	return
}

func (lib *library) getPathRelativeToLibraryRoot(absolutePath string) (relativePath string, insideLibraryDir bool) {
	relativePath, err := filepath.Rel(lib.rootPath, absolutePath)
	if err != nil || strings.HasPrefix(relativePath, "..") {
		relativePath = ""
		insideLibraryDir = false
		return
	}
	insideLibraryDir = true
	return
}

func (lib *library) ChdirToRoot() {
	err := os.Chdir(lib.rootPath)
	if err != nil {
		panic(err)
	}
}

func (files LibraryFiles) DisplayDelta(absoluteWorkingDirectory string) {

}
