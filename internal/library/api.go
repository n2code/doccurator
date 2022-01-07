package library

import (
	"io"

	. "github.com/n2code/doccinator/internal/document"
)

type LibraryDocument struct {
	id      DocumentId
	library *library
}

type PathStatus rune

const (
	Error     PathStatus = 'E'
	Tracked   PathStatus = '=' //no change
	Removed   PathStatus = 'X' //no change
	Duplicate PathStatus = '2' //uncritical, automatically resolvable
	Touched   PathStatus = '~' //uncritical, automatically resolvable
	Moved     PathStatus = '>' //uncritical, automatically resolvable
	Untracked PathStatus = '+' //action required
	Modified  PathStatus = '!' //action required
	Missing   PathStatus = '?' //action required
)

type CheckedPath struct {
	libraryPath string //relative to library root, system-native
	status      PathStatus
	matchingId  DocumentId
}

// LibraryApi expects absolute system-native paths (with respect to the directory separator)
type LibraryApi interface {
	CreateDocument(DocumentId) (LibraryDocument, error)
	SetDocumentPath(doc LibraryDocument, absolutePath string)
	GetDocumentByPath(absolutePath string) (doc LibraryDocument, exists bool)
	UpdateDocumentFromFile(LibraryDocument) error
	MarkDocumentAsRemoved(LibraryDocument)
	ForgetDocument(LibraryDocument)
	CheckFilePath(absolutePath string) (result CheckedPath, err error)
	Scan(skip func(absoluteFilePath string) bool) []CheckedPath
	SaveToLocalFile(path string, overwrite bool) error
	LoadFromLocalFile(path string)
	SetRoot(absolutePath string)
	GetRoot() string
	PrintAllRecords(io.Writer)
}

func MakeRuntimeLibrary() LibraryApi {
	return &library{
		documents:    make(map[DocumentId]DocumentApi),
		relPathIndex: make(map[string]DocumentApi)}
}
