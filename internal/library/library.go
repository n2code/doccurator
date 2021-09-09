package library

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	. "github.com/n2code/doccinator/internal/document"
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

type library struct {
	documents    map[DocumentId]*Document
	relPathIndex map[string]*Document
	rootPath     string // path has system-native directory separators
}

type LibraryDocument struct {
	id      DocumentId
	library *library
}

type LibraryFile struct {
	libraryPath string
	status      fileStatus
}

type LibraryFiles []LibraryFile

// The Library API expects absolute system-native paths (with respect to the directory separator)
type Library interface {
	CreateDocument(DocumentId) (LibraryDocument, error)
	SetDocumentPath(doc LibraryDocument, absolutePath string)
	GetDocumentByPath(string) (doc LibraryDocument, exists bool)
	UpdateDocumentFromFile(LibraryDocument) error
	//MarkDocumentAsRemoved(*Document) error
	ForgetDocument(LibraryDocument)
	Scan() (LibraryFiles, error)
	SaveToLocalFile(absolutePath string, overwrite bool)
	LoadFromLocalFile(absolutePath string)
	SetRoot(absolutePath string)
	ChdirToRoot()
	AllRecordsAsText() string
}

func MakeRuntimeLibrary() Library {
	return &library{
		documents:    make(map[DocumentId]*Document),
		relPathIndex: make(map[string]*Document)}
}

func (lib *library) CreateDocument(id DocumentId) (document LibraryDocument, err error) {
	if _, exists := lib.documents[id]; exists {
		err = fmt.Errorf("document ID %s already exists", id)
		return
	}
	lib.documents[id] = NewDocument(id)
	return LibraryDocument{id: id, library: lib}, nil
}

func (lib *library) SetDocumentPath(document LibraryDocument, absolutePath string) {
	newRelativePath, inLibrary := lib.getPathRelativeToLibraryRoot(absolutePath)
	if !inLibrary {
		panic("path not inside library")
	}
	doc := lib.documents[document.id] //caller error if nil
	delete(lib.relPathIndex, doc.Path())
	doc.SetPath(newRelativePath)
	lib.relPathIndex[newRelativePath] = doc
}

func (lib *library) GetDocumentByPath(absolutePath string) (document LibraryDocument, exists bool) {
	relativePath, inLibrary := lib.getPathRelativeToLibraryRoot(absolutePath)
	if !inLibrary {
		exists = false
		return
	}
	doc, exists := lib.relPathIndex[relativePath]
	if exists {
		document = LibraryDocument{id: doc.Id(), library: lib}
	}
	return
}

func (lib *library) UpdateDocumentFromFile(document LibraryDocument) error {
	doc := lib.documents[document.id] //caller error if nil
	absoluteLocation := filepath.Join(lib.rootPath, doc.Path())
	doc.UpdateFromFile(absoluteLocation)
	return nil
}

//func (lib *library) MarkDocumentAsRemoved(doc *Document) error {
//TODO
//}

func (lib *library) ForgetDocument(document LibraryDocument) {
	doc := lib.documents[document.id] //caller error if nil
	relativePath := doc.Path()
	delete(lib.relPathIndex, relativePath)
	delete(lib.documents, doc.Id())
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

func (lib *library) SetRoot(path string) {
	lib.rootPath = path
}

func (lib *library) ChdirToRoot() {
	err := os.Chdir(lib.rootPath)
	if err != nil {
		panic(err)
	}
}

type docsByRecordedAndId []*Document

func (l docsByRecordedAndId) Len() int {
	return len(l)
}
func (l docsByRecordedAndId) Swap(i, j int) {
	l[i], l[j] = l[j], l[i]
}
func (l docsByRecordedAndId) Less(i, j int) bool {
	return l[i].Recorded() < l[j].Recorded() || (l[i].Recorded() == l[j].Recorded() && l[i].Id() < l[j].Id())
}

func (lib *library) AllRecordsAsText() string {
	docList := make(docsByRecordedAndId, 0, len(lib.documents))
	for _, doc := range lib.documents {
		docList = append(docList, doc)
	}
	sort.Sort(docList)

	var builder strings.Builder
	for _, doc := range docList {
		fmt.Fprintln(&builder, doc)
	}
	return builder.String()
}

func (files LibraryFiles) DisplayDelta(absoluteWorkingDirectory string) {
	//TODO
}
