package library

import (
	"bufio"
	"crypto/sha256"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	checksum "crypto/sha256"
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

func (lib *library) GetObsoleteDocumentsForPath(absolutePath string) (retirees []Document) {
	retirees = make([]Document, 0, 1) //never return nil for result uniformity
	anchoredPath, inLibrary := lib.getAnchoredPath(absolutePath)
	if !inLibrary {
		return
	}
	for _, retired := range lib.getAllObsoleteDocumentsAt(anchoredPath) {
		retirees = append(retirees, Document{id: retired.Id(), library: lib})
	}
	return
}

//getAllObsoleteDocumentsAt returns all obsoleted documents that used to be present at the given path, nil if none exists
func (lib *library) getAllObsoleteDocumentsAt(anchored string) (retirees []document.Api) {
	//linear scan, could be improved
	for _, doc := range lib.documents {
		if doc.IsObsolete() && doc.AnchoredPath() == anchored {
			retirees = append(retirees, doc)
		}
	}
	return
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

// Absolutize turns an anchored path into an absolute one
func (lib *library) Absolutize(anchored string) string {
	return filepath.Join(lib.rootPath, anchored)
}

func calculateFileChecksum(path string) (sum [sha256.Size]byte, err error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return
	}
	sum = sha256.Sum256(content)
	return
}

func (lib *library) loadIgnoreFile(absoluteIgnoreFile string) (err error) {
	file, openErr := os.Open(absoluteIgnoreFile)
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

			//checks for cleanliness of path spec

			if path.IsAbs(line) {
				return fmt.Errorf("absolute path given")
			}
			if line == "." {
				return fmt.Errorf(`use "./" as path spec to ignore current directory`)
			}
			{
				rawPathParts := strings.Split(line, "/")
				for _, rawPart := range rawPathParts {
					if rawPart == ".." {
						return fmt.Errorf(`path spec contains ".." element`)
					}
					if rawPart == "." && line != "./" {
						return fmt.Errorf(`path spec contains superfluous "." element`)
					}
				}
			}

			//normalization

			nativeIgnored := filepath.FromSlash(line)
			absoluteIgnored := filepath.Join(filepath.Dir(absoluteIgnoreFile), nativeIgnored)
			anchoredIgnored, _ := filepath.Rel(lib.rootPath, absoluteIgnored) //error impossible because both are absolute and ".." does not occur
			if absoluteIgnored == lib.rootPath {                              //sanity check
				return fmt.Errorf("refers to library root dir")
			}

			//path spec acceptable:
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

func IsAnyFilterMatching(filters *[]PathSkipEvaluator, absolute string, isDir bool) bool {
	for _, filter := range *filters {
		if filter(absolute, isDir) {
			return true
		}
	}
	return false
}

func (lib *library) Scan(scanFilters []PathSkipEvaluator, resultFilters []PathSkipEvaluator, skipReadOnSizeMatch bool) (results []CheckedPath, hasNoErrors bool) {
	results = make([]CheckedPath, 0, len(lib.documents))
	coveredLibraryPaths := make(map[string]bool)
	hasNoErrors = true

	isFileResultFiltered := func(absolute string) bool {
		return IsAnyFilterMatching(&resultFilters, absolute, false)
	}
	addError := func(anchored string, err error) {
		results = append(results, CheckedPath{
			anchoredPath: anchored,
			status:       Error,
			err:          err,
		})
		hasNoErrors = false
	}

	visitor := func(absolutePath string, d fs.DirEntry, dirError error) error {
		if dirError != nil {
			badDir, _ := filepath.Rel(lib.rootPath, absolutePath)
			addError(badDir, fmt.Errorf("directory scan error: %w", dirError))
			return filepath.SkipDir //attempt to continue
		}

		isDir := d.IsDir()

		//attempt loading an ignore file (happens before ignore evaluation so a directory can ignore itself!)
		if isDir {
			ignoreFileCandidate := filepath.Join(absolutePath, IgnoreFileName)
			if _, err := os.Stat(ignoreFileCandidate); err == nil { //ignore file does not have to exist
				if ignoreErr := lib.loadIgnoreFile(ignoreFileCandidate); ignoreErr != nil {
					badIgnore, _ := filepath.Rel(lib.rootPath, ignoreFileCandidate)
					addError(badIgnore, fmt.Errorf("ignore file error: %w", ignoreErr))
				}
			}
		}

		//evaluate scan filters
		if lib.isIgnored(absolutePath, isDir) || IsAnyFilterMatching(&scanFilters, absolutePath, isDir) {
			if isDir {
				return filepath.SkipDir //to prevent descent
			}
			return nil //to continue scan with next candidate
		}

		//check file
		if !isDir {
			result := lib.CheckFilePath(absolutePath, skipReadOnSizeMatch)
			coveredLibraryPaths[result.anchoredPath] = true
			if !isFileResultFiltered(absolutePath) {
				results = append(results, result)
				if result.status == Error {
					hasNoErrors = false
				}
			}
		}

		return nil
	}
	filepath.WalkDir(lib.rootPath, visitor) //errors are communicated as entry in output parameter

	for _, doc := range lib.documents {
		if _, alreadyCheckedPath := coveredLibraryPaths[doc.AnchoredPath()]; !alreadyCheckedPath {
			absolutePath := lib.getAbsolutePathOfDocument(doc)
			if isFileResultFiltered(absolutePath) {
				continue
			}
			results = append(results, lib.CheckFilePath(absolutePath, skipReadOnSizeMatch))
		}
	}

	return
}

func (lib *library) getAnchoredPath(absolutePath string) (anchored string, insideLibraryDir bool) {
	anchored, err := filepath.Rel(lib.rootPath, absolutePath)
	if err != nil || strings.HasPrefix(anchored, "..") {
		insideLibraryDir = false
		return
	}
	insideLibraryDir = true
	return
}

func (lib *library) activePathExists(anchored string) (exists bool) {
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

func (libDoc *Document) RecordProperties() (size int64, modTime time.Time, sha256 [checksum.Size]byte) {
	doc := libDoc.library.documents[libDoc.id] //existence probe, caller error if any is nil
	size, modTimeUnix, sha256 := doc.RecordedFileProperties()
	modTime = time.Unix(int64(modTimeUnix), 0)
	return
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

	if libDoc.library.activePathExists(standardPath) {
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

func (s PathStatus) RepresentsChange() bool {
	return s != Tracked && s != Removed
}

func (s PathStatus) RepresentsWaste() bool {
	return s == Duplicate || s == Obsolete
}
