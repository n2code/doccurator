package document

import (
	checksum "crypto/sha256"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

func (doc *document) Id() DocumentId {
	return doc.id
}

func (doc *document) Recorded() unixTimestamp {
	return doc.recorded
}

func (doc *document) Changed() unixTimestamp {
	return doc.changed
}

func (doc *document) Removed() bool {
	return doc.removed
}

func (doc *document) SetRemoved() {
	if !doc.removed {
		doc.removed = true
		doc.updateRecordChangeDate()
	}
}

//Path returns a filepath relative to the library root directory
func (doc *document) Path() string {
	return doc.localStorage.pathRelativeToLibrary()
}

//SetPath expects a filepath relative to the library root directory
func (doc *document) SetPath(relativePath string) {
	doc.localStorage.setFromRelativePath(relativePath)
	doc.updateRecordChangeDate()
}

//reads and stats the document using the recorded document path and the library root (relative/absolute)
func (doc *document) UpdateFromFileOnStorage(libraryRoot string) (changed bool, err error) {
	path := filepath.Join(libraryRoot, doc.localStorage.pathRelativeToLibrary()) //path may be relative if the library root is relative
	statsChanged, err := doc.localStorage.updateFileStats(path)
	if err != nil {
		return false, err
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	contentChanged := doc.contentMetadata.setFromContent(content)
	changed = statsChanged || contentChanged
	if changed {
		doc.updateRecordChangeDate()
	}
	return
}

//Calculates file status using the recorded document path and the library root (relative/absolute)
func (doc *document) CompareToFileOnStorage(libraryRoot string) TrackedFileStatus {
	path := filepath.Join(libraryRoot, doc.localStorage.pathRelativeToLibrary())

	stat, err := os.Stat(path)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return AccessError
		}
		return NoFileFound
	}

	if stat.Size() != doc.contentMetadata.size {
		return ModifiedFile
	}
	//TODO [FEATURE]: introduce switch which skips the full read at this point for better performance
	content, err := os.ReadFile(path)
	if err != nil {
		return AccessError
	}
	if checksum.Sum256(content) != doc.contentMetadata.sha256Hash {
		return ModifiedFile
	}

	if unixTimestamp(stat.ModTime().Unix()) != doc.localStorage.lastModified {
		//TODO [FEATURE]: on performance optimization do a full read if last modified differs
		return TouchedFile
	}

	return UnmodifiedFile
}

func (doc *document) MatchesChecksum(sha256 [checksum.Size]byte) bool {
	return doc.contentMetadata.sha256Hash == sha256
}

func (doc *document) updateRecordChangeDate() {
	doc.changed = unixTimestamp(time.Now().Unix())
}

func (stored *storedFile) setFromRelativePath(relativePath string) {
	stored.directory = SemanticPathFromNative(filepath.Dir(relativePath))
	stored.name = filepath.Base(relativePath)
}

func (stored *storedFile) pathRelativeToLibrary() string {
	return filepath.Join(stored.directory.ToNativeFilepath(), stored.name)
}

func (stored *storedFile) updateFileStats(path string) (hasChanged bool, err error) {
	stat, err := os.Stat(path)
	if err != nil {
		return
	}
	oldLastModified := stored.lastModified
	stored.lastModified = unixTimestamp(stat.ModTime().Unix())
	if stored.lastModified != oldLastModified {
		hasChanged = true
	}
	return
}

func (meta *contentMetadata) setFromContent(content []byte) (hasChanged bool) {
	oldSize := meta.size
	oldHash := meta.sha256Hash
	meta.size = int64(len(content))
	meta.sha256Hash = checksum.Sum256(content)
	if meta.size != oldSize || meta.sha256Hash != oldHash {
		hasChanged = true
	}
	return
}
