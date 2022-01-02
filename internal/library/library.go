package library

import (
	"crypto/sha256"
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

func (lib *library) MarkDocumentAsRemoved(document LibraryDocument) {
	doc := lib.documents[document.id] //caller error if nil
	doc.SetRemoved()
}

func (lib *library) ForgetDocument(document LibraryDocument) {
	doc := lib.documents[document.id] //caller error if nil
	relativePath := doc.Path()
	delete(lib.relPathIndex, relativePath)
	delete(lib.documents, doc.Id())
}

//CheckPath requires the current working directory to be the library root.
// It deals with all combinations of the given path being on record and/or [not] existing in reality.
func (lib *library) CheckFilePath(absolutePath string) (result CheckedPath, err error) {
	result.status = Unknown

	var inLibrary bool
	result.libraryPath, inLibrary = lib.getPathRelativeToLibraryRoot(absolutePath)
	if !inLibrary {
		result.status = Error
		err = fmt.Errorf("path is not below library root: %s", absolutePath)
		return
	}

	if doc, isOnRecord := lib.relPathIndex[result.libraryPath]; isOnRecord {
		switch status := doc.VerifyRecordedFileStatus(); status {
		case UnmodifiedFile:
			result.status = Tracked
		case TouchedFile:
			result.status = Touched
		case ModifiedFile:
			result.status = Modified
		case RemovedFile:
			result.status = Removed
		case MissingFile:
			result.status = Missing
		case ZombieFile:
			result.status = Untracked
		}
		return
	}

	//path not on record

	fileChecksum, err := calculateFileChecksum(result.libraryPath)
	if err != nil {
		result.status = Error
		return
	}

	result.status = Untracked
	for _, doc := range lib.documents {
		if doc.MatchesChecksum(fileChecksum) {
			switch doc.VerifyRecordedFileStatus() {
			case MissingFile:
				result.status = Moved
				break
			case UnmodifiedFile, TouchedFile:
				result.status = Duplicate
				break
			}
		}
	}
	return
}

func calculateFileChecksum(relativePath string) (sum [sha256.Size]byte, err error) {
	content, err := os.ReadFile(relativePath)
	if err != nil {
		return
	}
	sum = sha256.Sum256(content)
	return
}

//Scan requires the current working directory to be the library root.
func (lib *library) Scan() (paths []CheckedPath) {
	paths = make([]CheckedPath, 0, len(lib.documents))
	coveredLibraryPaths := make(map[string]bool)

	visitor := func(absolutePath string, d fs.DirEntry, walkError error) error {
		if walkError != nil {
			return walkError
		}
		if !d.IsDir() {
			libPath, _ := lib.CheckFilePath(absolutePath)
			paths = append(paths, libPath)
			coveredLibraryPaths[libPath.libraryPath] = true
			//TODO: provide check path error output
			// if err != nil {
			// 	fmt.Println
			// }
		}
		return nil
	}
	filepath.WalkDir(lib.rootPath, visitor)

	for _, doc := range lib.documents {
		if _, alreadyChecked := coveredLibraryPaths[doc.Path()]; !alreadyChecked {
			absolutePath := lib.getAbsolutePathOfDocument(doc)
			libPath, _ := lib.CheckFilePath(absolutePath)
			paths = append(paths, libPath)
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

func (lib *library) SetRoot(path string) {
	lib.rootPath = path
}

func (lib *library) ChdirToRoot() {
	err := os.Chdir(lib.rootPath)
	if err != nil {
		panic(err)
	}
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

func (lib *library) AllRecordsAsText() string {
	docList := make(docsByRecordedAndId, 0, len(lib.documents))
	for _, doc := range lib.documents {
		if !doc.Removed() {
			docList = append(docList, doc)
		}
	}
	sort.Sort(docList)

	var builder strings.Builder
	for _, doc := range docList {
		fmt.Fprintln(&builder, doc)
	}
	return builder.String()
}

func (libDoc *LibraryDocument) IsRemoved() bool {
	doc := libDoc.library.documents[libDoc.id] //caller error if any is nil
	return doc.Removed()
}

func (p CheckedPath) Status() PathStatus {
	return p.status
}

func (p CheckedPath) PathRelativeToLibraryRoot() string {
	return p.libraryPath
}
