package library

import (
	"bufio"
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
	newAnchoredPath, inLibrary := lib.getAnchoredPath(absolutePath)
	if !inLibrary {
		return fmt.Errorf("path outside library: %s", absolutePath)
	}
	doc := lib.documents[ref.id] //caller error if nil
	if conflicting, pathAlreadyKnown := lib.activeAnchoredPathIndex[newAnchoredPath]; pathAlreadyKnown && conflicting.Id() != doc.Id() {
		return fmt.Errorf("document %s already exists for path %s", conflicting.Id(), absolutePath)
	}
	if filepath.Base(absolutePath) == LocatorFileName {
		return fmt.Errorf("locator files must not be added to the library")
	}
	if !doc.IsObsolete() {
		delete(lib.activeAnchoredPathIndex, doc.AnchoredPath())
		lib.activeAnchoredPathIndex[newAnchoredPath] = doc
	}
	doc.SetPath(newAnchoredPath)
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
	anchoredPath, inLibrary := lib.getAnchoredPath(absolutePath)
	if !inLibrary {
		exists = false
		return
	}
	doc, exists := lib.activeAnchoredPathIndex[anchoredPath]
	if exists {
		ref = Document{id: doc.Id(), library: lib}
	}
	return
}

func (lib *library) ObsoleteDocumentExistsForPath(absolutePath string) bool {
	anchoredPath, inLibrary := lib.getAnchoredPath(absolutePath)
	if !inLibrary {
		return false
	}
	//linear scan, could be improved
	for _, doc := range lib.documents {
		if doc.IsObsolete() && doc.AnchoredPath() == anchoredPath {
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
		delete(lib.activeAnchoredPathIndex, doc.AnchoredPath())
	}
}

func (lib *library) ForgetDocument(ref Document) {
	doc := lib.documents[ref.id] //caller error if nil
	if !doc.IsObsolete() {
		anchored := doc.AnchoredPath()
		delete(lib.activeAnchoredPathIndex, anchored)
	}
	delete(lib.documents, doc.Id())
}

//deals with all combinations of the given path being on record and/or [not] existing in reality.
func (lib *library) CheckFilePath(absolutePath string, skipReadOnSizeMatch bool) (result CheckedPath) {
	result.status = Untracked

	defer func() {
		if result.err != nil {
			result.status = Error
		}
	}()

	var inLibrary bool
	result.anchoredPath, inLibrary = lib.getAnchoredPath(absolutePath)
	if !inLibrary {
		result.err = fmt.Errorf("path is not below library root: %s", absolutePath)
		return
	}

	if doc, isOnActiveRecord := lib.activeAnchoredPathIndex[result.anchoredPath]; isOnActiveRecord {
		switch status := doc.CompareToFileOnStorage(lib.rootPath, skipReadOnSizeMatch); status {
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
			result.err = fmt.Errorf("could not access last known location (%s) of document %s", doc.AnchoredPath(), doc.Id())
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
			statusOfContentMatch := doc.CompareToFileOnStorage(lib.rootPath, skipReadOnSizeMatch)
			switch statusOfContentMatch {
			case document.UnmodifiedFile, document.TouchedFile:
				foundMatchingActive = true
			case document.ModifiedFile:
				foundModifiedActive = true
			case document.NoFileFound:
				foundMissingActive = true
				anyMissingMatchingActive = doc
			case document.FileAccessError:
				result.err = fmt.Errorf("could not access last known location (%s) of document %s", doc.AnchoredPath(), doc.Id())
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

func (lib *library) loadIgnoreFile(absolutePath string) (err error) {
	file, openErr := os.Open(absolutePath)
	if openErr != nil {
		return openErr
	}
	defer file.Close()

	lineScanner := bufio.NewScanner(file) //splits by newline by default
	var line string
	lineNumber := 0
	defer func() {
		if err != nil {
			err = fmt.Errorf("line %d (%s): %w", lineNumber, line, err)
		}
	}()

	for lineScanner.Scan() {
		line = lineScanner.Text()
		lineNumber++
		if len(line) > 0 && !strings.HasPrefix(line, "#") {
			if strings.HasPrefix(line, "/") {
				return fmt.Errorf("absolute path given")
			}
			nativePath := filepath.FromSlash(line)
			absoluteIgnored := filepath.Join(filepath.Dir(absolutePath), nativePath)
			anchoredIgnored, relErr := filepath.Rel(lib.rootPath, absoluteIgnored)
			if relErr != nil {
				return fmt.Errorf("relativizing failed (%w)", relErr)
			}
			if anchoredIgnored == "." {
				return fmt.Errorf("refers to directory itself")
			}
			if components := strings.Split(anchoredIgnored, string(filepath.Separator)); len(components) == 0 || components[0] == ".." {
				return fmt.Errorf("path refers to directory above")
			}
			lib.ignoredPaths[ignoredLibraryPath{
				anchored:  anchoredIgnored,
				directory: strings.HasSuffix(line, "/"),
			}] = true

		}
	}
	if scanErr := lineScanner.Err(); scanErr != nil {
		return scanErr
	}
	return nil
}

func (lib *library) isIgnored(absolutePath string, isDir bool) bool {
	if filepath.Base(absolutePath) == LocatorFileName {
		return true
	}
	anchoredPath, _ := filepath.Rel(lib.rootPath, absolutePath)
	return lib.ignoredPaths[ignoredLibraryPath{anchored: anchoredPath, directory: isDir}]
}

func (lib *library) Scan(additionalSkip func(absoluteFilePath string) bool, skipReadOnSizeMatch bool) (paths []CheckedPath, hasNoErrors bool) {
	paths = make([]CheckedPath, 0, len(lib.documents))
	hasNoErrors = true

	coveredLibraryPaths := make(map[string]bool)
	movedIds := make(map[document.Id]bool)

	visitor := func(absolutePath string, d fs.DirEntry, walkError error) error {
		if walkError != nil {
			badPath, _ := filepath.Rel(lib.rootPath, absolutePath)
			paths = append(paths, CheckedPath{
				anchoredPath: badPath,
				status:       Error,
				err:          fmt.Errorf("scan aborted: %w", walkError),
			})
			hasNoErrors = false
			return walkError
		}
		if d.IsDir() { //attempt loading an ignore file
			ignoreFileCandidate := filepath.Join(absolutePath, IgnoreFileName)
			if _, err := os.Stat(ignoreFileCandidate); err == nil {
				if ignoreErr := lib.loadIgnoreFile(ignoreFileCandidate); ignoreErr != nil {
					badIgnore, _ := filepath.Rel(lib.rootPath, ignoreFileCandidate)
					paths = append(paths, CheckedPath{
						anchoredPath: badIgnore,
						status:       Error,
						err:          fmt.Errorf("ignore file error: %w", ignoreErr),
					})
					hasNoErrors = false
				}
			}
		}
		if lib.isIgnored(absolutePath, d.IsDir()) || additionalSkip(absolutePath) {
			if d.IsDir() {
				return filepath.SkipDir //to prevent descent
			}
			return nil //to continue scan with next candidate
		}
		if !d.IsDir() {
			libPath := lib.CheckFilePath(absolutePath, skipReadOnSizeMatch)
			paths = append(paths, libPath)
			coveredLibraryPaths[libPath.anchoredPath] = true
			switch libPath.status {
			case Moved:
				movedIds[libPath.referencing.id] = true
			case Error:
				hasNoErrors = false
			}
		}
		return nil
	}
	filepath.WalkDir(lib.rootPath, visitor) //errors are communicated as path

	for _, doc := range lib.documents {
		if _, alreadyCheckedPath := coveredLibraryPaths[doc.AnchoredPath()]; !alreadyCheckedPath {
			if _, alreadyCoveredId := movedIds[doc.Id()]; !alreadyCoveredId {
				absolutePath := lib.getAbsolutePathOfDocument(doc)
				libPath := lib.CheckFilePath(absolutePath, skipReadOnSizeMatch)
				paths = append(paths, libPath)
			}
		}
	}

	return
}

func (lib *library) getAnchoredPath(absolutePath string) (anchored string, insideLibraryDir bool) {
	anchored, err := filepath.Rel(lib.rootPath, absolutePath)
	if err != nil || strings.HasPrefix(anchored, "..") {
		anchored = ""
		insideLibraryDir = false
		return
	}
	insideLibraryDir = true
	return
}

func (lib *library) pathExists(anchored string) (exists bool) {
	_, exists = lib.activeAnchoredPathIndex[anchored]
	return
}

func (lib *library) getAbsolutePathOfDocument(doc document.Api) string {
	return filepath.Join(lib.rootPath, doc.AnchoredPath())
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

func (libDoc *Document) Id() document.Id {
	doc := libDoc.library.documents[libDoc.id] //existence probe, caller error if any is nil
	return doc.Id()
}

func (libDoc *Document) IsObsolete() bool {
	doc := libDoc.library.documents[libDoc.id] //caller error if any is nil
	return doc.IsObsolete()
}

func (libDoc *Document) AnchoredPath() string {
	doc := libDoc.library.documents[libDoc.id] //caller error if any is nil
	return doc.AnchoredPath()
}

func (libDoc *Document) RenameToStandardNameFormat(dryRun bool) (newNameIfDifferent string, err error, fsRollback func() error) {
	fsRollback = func() error { return nil }

	//calculate new name and paths

	doc := libDoc.library.documents[libDoc.id] //caller error if any is nil
	standardName := doc.StandardizedFilename()
	oldPath := doc.AnchoredPath()
	standardPath := filepath.Join(filepath.Dir(oldPath), standardName)
	if standardPath == oldPath {
		return
	}
	newNameIfDifferent = standardName
	absoluteOldPath := filepath.Join(libDoc.library.GetRoot(), oldPath)
	absoluteNewPath := filepath.Join(libDoc.library.GetRoot(), standardPath)

	//check for conflicts

	if libDoc.library.pathExists(standardPath) {
		err = fmt.Errorf("standardized path already on record: %s", absoluteNewPath)
		return
	}
	if _, statErr := os.Stat(absoluteNewPath); statErr == nil {
		err = fmt.Errorf("file with standardized name already exists: %s", absoluteNewPath)
		return
	}

	//early exit if caller is only interested in change preview
	if dryRun {
		return
	}

	//apply rename

	if err = libDoc.library.SetDocumentPath(*libDoc, absoluteNewPath); err != nil {
		return
	}
	if err = os.Rename(absoluteOldPath, absoluteNewPath); err != nil {
		return
	}
	fsRollback = func(source string, target string) func() error {
		return func() error {
			return os.Rename(source, target)
		}
	}(absoluteNewPath, absoluteOldPath)

	return
}

func (p CheckedPath) Status() PathStatus {
	return p.status
}

func (p CheckedPath) AnchoredPath() string {
	return p.anchoredPath
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
