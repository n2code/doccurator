package library

import (
	. "github.com/n2code/doccinator/internal/document"
)

type LibraryDocument struct {
	id      DocumentId
	library *library
}

type PathStatus rune

const (
	Error     PathStatus = 'E'
	Unknown   PathStatus = '?'
	Untracked PathStatus = '+'
	Tracked   PathStatus = '='
	Touched   PathStatus = '~'
	Modified  PathStatus = '!'
	Moved     PathStatus = '>'
	Removed   PathStatus = 'X'
	Missing   PathStatus = '∅'
	Duplicate PathStatus = '2'
)

type CheckedPath struct {
	libraryPath string
	status      PathStatus
}

// The Library API expects absolute system-native paths (with respect to the directory separator)
type Library interface {
	CreateDocument(DocumentId) (LibraryDocument, error)
	SetDocumentPath(doc LibraryDocument, absolutePath string)
	GetDocumentByPath(absolutePath string) (doc LibraryDocument, exists bool)
	UpdateDocumentFromFile(LibraryDocument) error
	MarkDocumentAsRemoved(LibraryDocument)
	ForgetDocument(LibraryDocument)
	CheckFilePath(absolutePath string) (result CheckedPath, err error)
	Scan() []CheckedPath
	SaveToLocalFile(absolutePath string, overwrite bool)
	LoadFromLocalFile(absolutePath string)
	SetRoot(absolutePath string)
	ChdirToRoot()
	AllRecordsAsText() string
}

func MakeRuntimeLibrary() Library {
	return &library{
		documents:    make(map[DocumentId]DocumentApi),
		relPathIndex: make(map[string]DocumentApi)}
}
