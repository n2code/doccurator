package library

import (
	"github.com/n2code/doccurator/internal/document"
)

type Document struct {
	id      document.Id
	library *library
}

const LocatorFileName = ".doccurator"

type PathStatus rune

const (
	Tracked   PathStatus = '=' //path on record, content matches record => okay
	Touched   PathStatus = '~' //path on record, content matches record, time does not => uncritical, auto mode updates time
	Modified  PathStatus = '!' //path on record but content differs => decision required, auto mode may record changes
	Missing   PathStatus = '?' //path on record, no file present => action required, not automatically resolvable
	Removed   PathStatus = '-' //path on record, obsolete, no file present => okay
	Duplicate PathStatus = '2' //path not on record, content already on record & present elsewhere => decision required, auto mode may delete file
	Moved     PathStatus = '>' //path not on record but content matches other missing path on record => uncritical, auto mode updates path
	Untracked PathStatus = '+' //path not on record, content not on record => uncritical, auto mode records file
	Error     PathStatus = 'E' //path not accessible as needed => action required, not automatically resolvable
	//Obsolete signifies either path on record and marked as obsolete with matching content
	//or path not on record and content is obsolete everywhere => decision required, auto mode may delete file
	Obsolete PathStatus = 'X'
)

type CheckedPath struct {
	libraryPath string //relative to library root, system-native
	status      PathStatus
	referencing Document
	err         error
}

// Api expects absolute system-native paths (with respect to the directory separator)
type Api interface {
	CreateDocument(document.Id) (Document, error)
	SetDocumentPath(doc Document, absolutePath string) error
	GetDocumentById(document.Id) (doc Document, exists bool)
	GetActiveDocumentByPath(absolutePath string) (doc Document, exists bool)
	UpdateDocumentFromFile(Document) (changed bool, err error)
	MarkDocumentAsObsolete(Document)
	ObsoleteDocumentExistsForPath(absolutePath string) bool
	ForgetDocument(Document)
	CheckFilePath(absolutePath string) CheckedPath
	Scan(skip func(absoluteFilePath string) bool) (paths []CheckedPath, hasNoErrors bool)
	SaveToLocalFile(path string, overwrite bool) error
	LoadFromLocalFile(path string)
	SetRoot(absolutePath string)
	GetRoot() string
	VisitAllRecords(func(Document)) //the list of visited documents is stable and isolated from changes during the visits
}

func NewLibrary() Api {
	return &library{
		documents:          make(map[document.Id]document.Api),
		relPathActiveIndex: make(map[string]document.Api)}
}
