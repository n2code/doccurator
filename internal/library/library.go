package library

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	. "github.com/n2code/doccinator/internal/document"
)

func (lib *library) CreateDocument(id DocumentId) (document LibraryDocument, err error) {
	if _, exists := lib.documents[id]; exists {
		err = fmt.Errorf("document ID %s already exists", id)
		return
	}
	lib.documents[id] = NewDocument(id)
	return LibraryDocument{id: id, library: lib}, nil
}

func (lib *library) SetDocumentPath(document LibraryDocument, absolutePath string) error {
	newRelativePath, inLibrary := lib.getPathRelativeToLibraryRoot(absolutePath)
	if !inLibrary {
		return fmt.Errorf("path outside library: %s", absolutePath)
	}
	doc := lib.documents[document.id] //caller error if nil
	if !doc.IsObsolete() {
		delete(lib.relPathActiveIndex, doc.Path())
		lib.relPathActiveIndex[newRelativePath] = doc
	}
	doc.SetPath(newRelativePath)
	return nil
}

func (lib *library) GetDocumentById(id DocumentId) (doc LibraryDocument, exists bool) {
	_, exists = lib.documents[id]
	if !exists {
		return LibraryDocument{}, false
	}
	return LibraryDocument{id: id, library: lib}, true
}

func (lib *library) GetActiveDocumentByPath(absolutePath string) (document LibraryDocument, exists bool) {
	relativePath, inLibrary := lib.getPathRelativeToLibraryRoot(absolutePath)
	if !inLibrary {
		exists = false
		return
	}
	doc, exists := lib.relPathActiveIndex[relativePath]
	if exists {
		document = LibraryDocument{id: doc.Id(), library: lib}
	}
	return
}

func (lib *library) ObsoleteDocumentExistsForPath(absolutePath string) bool {
	relativePath, inLibrary := lib.getPathRelativeToLibraryRoot(absolutePath)
	if !inLibrary {
		return false
	}
	//linear scan, could be improved
	for _, doc := range lib.documents {
		if doc.IsObsolete() && doc.Path() == relativePath {
			return true
		}
	}
	return false
}

func (lib *library) UpdateDocumentFromFile(document LibraryDocument) (changed bool, err error) {
	doc := lib.documents[document.id] //caller error if nil
	return doc.UpdateFromFileOnStorage(lib.rootPath)
}

func (lib *library) MarkDocumentAsObsolete(document LibraryDocument) {
	doc := lib.documents[document.id] //caller error if nil
	if !doc.IsObsolete() {
		doc.DeclareObsolete()
		delete(lib.relPathActiveIndex, doc.Path())
	}
}

func (lib *library) ForgetDocument(document LibraryDocument) {
	doc := lib.documents[document.id] //caller error if nil
	if !doc.IsObsolete() {
		relativePath := doc.Path()
		delete(lib.relPathActiveIndex, relativePath)
	}
	delete(lib.documents, doc.Id())
}

//deals with all combinations of the given path being on record and/or [not] existing in reality.
func (lib *library) CheckFilePath(absolutePath string) (result CheckedPath) {
	result.status = Untracked

	defer func() {
		if result.err != nil {
			result.status = Error
		}
	}()

	var inLibrary bool
	result.libraryPath, inLibrary = lib.getPathRelativeToLibraryRoot(absolutePath)
	if !inLibrary {
		result.err = fmt.Errorf("path is not below library root: %s", absolutePath)
		return
	}

	if doc, isOnActiveRecord := lib.relPathActiveIndex[result.libraryPath]; isOnActiveRecord {
		// result.matchingId = doc.Id() //TODO: justify and activate
		switch status := doc.CompareToFileOnStorage(lib.rootPath); status {
		case UnmodifiedFile:
			result.status = Tracked
		case TouchedFile:
			result.status = Touched
		case ModifiedFile:
			result.status = Modified
		case NoFileFound:
			result.status = Missing
		case AccessError:
			result.err = fmt.Errorf("could not access last known location (%s) of document %s", doc.Path(), doc.Id())
		}
		return
	}

	//path not on active record => inspect

	stat, statErr := os.Stat(absolutePath)
	if statErr != nil {
		switch {
		case !errors.Is(statErr, fs.ErrNotExist):
			result.err = statErr
		case lib.ObsoleteDocumentExistsForPath(absolutePath):
			result.status = Removed //because path does not exist anymore, as expected
		default:
			result.err = fmt.Errorf("path does not exist: %s", absolutePath)
		}
		return
	}
	if stat.IsDir() {
		result.err = fmt.Errorf("path is not a file: %s", absolutePath)
		return
	}
	fileChecksum, checksumErr := calculateFileChecksum(absolutePath)
	if checksumErr != nil {
		result.err = checksumErr
		return
	}

	//file exists that is not on active record, match to known contents by checksum

	foundMatchingActive := false
	foundModifiedActive := false
	foundMissingActive := false
	foundMatchingObsolete := false

	for _, doc := range lib.documents {
		if doc.MatchesChecksum(fileChecksum) {
			if doc.IsObsolete() {
				foundMatchingObsolete = true
				continue
			}
			statusOfContentMatch := doc.CompareToFileOnStorage(lib.rootPath)
			switch statusOfContentMatch {
			case UnmodifiedFile, TouchedFile:
				foundMatchingActive = true
			case ModifiedFile:
				foundModifiedActive = true
			case NoFileFound:
				foundMissingActive = true
			case AccessError:
				result.err = fmt.Errorf("could not access last known location (%s) of document %s", doc.Path(), doc.Id())
				return
			}
		}
	}

	result.status = Untracked
	switch {
	case foundMissingActive:
		result.status = Moved
	case foundMatchingActive:
		if foundMatchingObsolete {
			result.status = Obsolete
		} else {
			result.status = Duplicate
		}
	case foundModifiedActive:
		if foundMatchingObsolete {
			result.status = Obsolete
		}
	case foundMatchingObsolete:
		result.status = Obsolete
	}

	return
}

func calculateFileChecksum(path string) (sum [sha256.Size]byte, err error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return
	}
	sum = sha256.Sum256(content)
	return
}

func (lib *library) Scan(skip func(absolutePath string) bool) (paths []CheckedPath, hasNoErrors bool) {
	paths = make([]CheckedPath, 0, len(lib.documents))
	hasNoErrors = true

	coveredLibraryPaths := make(map[string]bool)
	movedIds := make(map[DocumentId]bool)

	visitor := func(absolutePath string, d fs.DirEntry, walkError error) error {
		if skip(absolutePath) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if walkError != nil {
			return walkError
		}
		if !d.IsDir() {
			libPath := lib.CheckFilePath(absolutePath)
			paths = append(paths, libPath)
			coveredLibraryPaths[libPath.libraryPath] = true
			switch libPath.status {
			case Moved:
				movedIds[libPath.matchingId] = true
			case Error:
				hasNoErrors = false
			}
		}
		return nil
	}
	filepath.WalkDir(lib.rootPath, visitor)

	for _, doc := range lib.documents {
		if _, alreadyCheckedPath := coveredLibraryPaths[doc.Path()]; !alreadyCheckedPath {
			if _, alreadyCoveredId := movedIds[doc.Id()]; !alreadyCoveredId {
				absolutePath := lib.getAbsolutePathOfDocument(doc)
				libPath := lib.CheckFilePath(absolutePath)
				paths = append(paths, libPath)
			}
		}
	}

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

func (lib *library) getAbsolutePathOfDocument(doc DocumentApi) string {
	return filepath.Join(lib.rootPath, doc.Path())
}

func (lib *library) SetRoot(absolutePath string) {
	lib.rootPath = absolutePath
}

//yields absolute path
func (lib *library) GetRoot() string {
	return lib.rootPath
}

func (l docsByRecordedAndId) Len() int {
	return len(l)
}
func (l docsByRecordedAndId) Swap(i, j int) {
	l[i], l[j] = l[j], l[i]
}
func (l docsByRecordedAndId) Less(i, j int) bool {
	return l[i].Recorded() < l[j].Recorded() || (l[i].Recorded() == l[j].Recorded() && l[i].Id() < l[j].Id())
}

func (lib *library) VisitAllRecords(visitor func(LibraryDocument)) {
	docList := make(docsByRecordedAndId, 0, len(lib.documents))
	for _, doc := range lib.documents {
		docList = append(docList, doc)
	}
	sort.Sort(docList)

	for _, doc := range docList {
		visitor(LibraryDocument{id: doc.Id(), library: lib})
	}
}

func (libDoc *LibraryDocument) IsObsolete() bool {
	doc := libDoc.library.documents[libDoc.id] //caller error if any is nil
	return doc.IsObsolete()
}

func (libDoc *LibraryDocument) PathRelativeToLibraryRoot() string {
	doc := libDoc.library.documents[libDoc.id] //caller error if any is nil
	return doc.Path()
}

func (p CheckedPath) Status() PathStatus {
	return p.status
}

func (p CheckedPath) PathRelativeToLibraryRoot() string {
	return p.libraryPath
}

func (p CheckedPath) GetError() error {
	return p.err
}

func (s PathStatus) RepresentsChange() bool {
	return s != Tracked && s != Removed
}
