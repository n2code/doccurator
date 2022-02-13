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

	"github.com/n2code/doccurator/internal/document"
)

func (lib *library) CreateDocument(id document.Id) (Document, error) {
	if id == document.MissingId {
		return Document{}, fmt.Errorf("document ID %s must not be used", id)
	}
	if _, exists := lib.documents[id]; exists {
		return Document{}, fmt.Errorf("document ID %s already exists", id)
	}
	lib.documents[id] = document.NewDocument(id)
	return Document{id: id, library: lib}, nil
}

func (lib *library) SetDocumentPath(ref Document, absolutePath string) error {
	newRelativePath, inLibrary := lib.getPathRelativeToLibraryRoot(absolutePath)
	if !inLibrary {
		return fmt.Errorf("path outside library: %s", absolutePath)
	}
	doc := lib.documents[ref.id] //caller error if nil
	if conflicting, pathAlreadyKnown := lib.relPathActiveIndex[newRelativePath]; pathAlreadyKnown && conflicting.Id() != doc.Id() {
		return fmt.Errorf("document %s already exists for path %s", conflicting.Id(), absolutePath)
	}
	if !doc.IsObsolete() {
		delete(lib.relPathActiveIndex, doc.Path())
		lib.relPathActiveIndex[newRelativePath] = doc
	}
	if filepath.Base(absolutePath) == LocatorFileName {
		return fmt.Errorf("locator files must not be added to the library")
	}
	doc.SetPath(newRelativePath)
	return nil
}

func (lib *library) GetDocumentById(id document.Id) (doc Document, exists bool) {
	_, exists = lib.documents[id]
	if !exists {
		return Document{}, false
	}
	return Document{id: id, library: lib}, true
}

func (lib *library) GetActiveDocumentByPath(absolutePath string) (ref Document, exists bool) {
	relativePath, inLibrary := lib.getPathRelativeToLibraryRoot(absolutePath)
	if !inLibrary {
		exists = false
		return
	}
	doc, exists := lib.relPathActiveIndex[relativePath]
	if exists {
		ref = Document{id: doc.Id(), library: lib}
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

func (lib *library) UpdateDocumentFromFile(ref Document) (changed bool, err error) {
	doc := lib.documents[ref.id] //caller error if nil
	return doc.UpdateFromFileOnStorage(lib.rootPath)
}

func (lib *library) MarkDocumentAsObsolete(ref Document) {
	doc := lib.documents[ref.id] //caller error if nil
	if !doc.IsObsolete() {
		doc.DeclareObsolete()
		delete(lib.relPathActiveIndex, doc.Path())
	}
}

func (lib *library) ForgetDocument(ref Document) {
	doc := lib.documents[ref.id] //caller error if nil
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
		switch status := doc.CompareToFileOnStorage(lib.rootPath); status {
		case document.UnmodifiedFile:
			result.status = Tracked
		case document.TouchedFile:
			result.status = Touched
			result.referencing = Document{id: doc.Id(), library: lib}
		case document.ModifiedFile:
			result.status = Modified
			result.referencing = Document{id: doc.Id(), library: lib}
		case document.NoFileFound:
			result.status = Missing
		case document.FileAccessError:
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

	var anyMissingMatchingActive document.Api
	for _, doc := range lib.documents {
		if doc.MatchesChecksum(fileChecksum) {
			if doc.IsObsolete() {
				foundMatchingObsolete = true
				continue
			}
			statusOfContentMatch := doc.CompareToFileOnStorage(lib.rootPath)
			switch statusOfContentMatch {
			case document.UnmodifiedFile, document.TouchedFile:
				foundMatchingActive = true
			case document.ModifiedFile:
				foundModifiedActive = true
			case document.NoFileFound:
				foundMissingActive = true
				anyMissingMatchingActive = doc
			case document.FileAccessError:
				result.err = fmt.Errorf("could not access last known location (%s) of document %s", doc.Path(), doc.Id())
				return
			}
		}
	}

	result.status = Untracked
	switch {
	case foundMissingActive:
		result.status = Moved
		result.referencing = Document{id: anyMissingMatchingActive.Id(), library: lib}
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
	movedIds := make(map[document.Id]bool)

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
				movedIds[libPath.referencing.id] = true
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

func (lib *library) getAbsolutePathOfDocument(doc document.Api) string {
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

func (lib *library) VisitAllRecords(visitor func(Document)) {
	docList := make(docsByRecordedAndId, 0, len(lib.documents))
	for _, doc := range lib.documents {
		docList = append(docList, doc)
	}
	sort.Sort(docList)

	for _, doc := range docList {
		visitor(Document{id: doc.Id(), library: lib})
	}
}

func (libDoc *Document) IsObsolete() bool {
	doc := libDoc.library.documents[libDoc.id] //caller error if any is nil
	return doc.IsObsolete()
}

func (libDoc *Document) PathRelativeToLibraryRoot() string {
	doc := libDoc.library.documents[libDoc.id] //caller error if any is nil
	return doc.Path()
}

func (libDoc *Document) RenameToStandardNameFormat() (newNameIfDifferent string, err error, fsRollback func() error) {
	fsRollback = func() error { return nil }
	doc := libDoc.library.documents[libDoc.id] //caller error if any is nil
	standardName, err := doc.StandardizedFilename()
	if err != nil {
		return
	}
	oldPath := doc.Path()
	standardPath := filepath.Join(filepath.Dir(oldPath), standardName)
	if standardPath == oldPath {
		return
	}
	newNameIfDifferent = standardName
	absoluteOldPath := filepath.Join(libDoc.library.GetRoot(), oldPath)
	absoluteNewPath := filepath.Join(libDoc.library.GetRoot(), standardPath)
	if _, err = os.Stat(absoluteNewPath); err == nil {
		err = fmt.Errorf("file with standardized name already exists: %s", absoluteNewPath)
		return
	}
	err = os.Rename(absoluteOldPath, absoluteNewPath)
	if err == nil {
		libDoc.library.SetDocumentPath(*libDoc, absoluteNewPath)
		fsRollback = func(source string, target string) func() error {
			return func() error {
				return os.Rename(source, target)
			}
		}(absoluteNewPath, absoluteOldPath)
	}
	return
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

func (p CheckedPath) ReferencedDocument() Document {
	return p.referencing
}

func (s PathStatus) RepresentsChange() bool {
	return s != Tracked && s != Removed
}
