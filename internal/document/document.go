package document

import (
	"crypto/sha256"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

func (doc *document) Id() DocumentId {
	return doc.id
}

//SetId allows ID restoration after deserialization and thus does not affect the change timestamp
func (doc *document) SetId(id DocumentId) {
	if doc.id != missingId {
		panic("ID change attempt")
	}
	doc.id = id
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
	doc.removed = true
}

//Path returns a filepath relative to the library root directory
func (doc *document) Path() string {
	return doc.localStorage.pathRelativeToLibrary()
}

//SetPath expects a filepath relative to the library root directory
func (doc *document) SetPath(relativePath string) {
	doc.localStorage.setFromRelativePath(relativePath)
	doc.updateChangeDate()
}

//reads and stats the document using the recorded document path and the library root (relative/absolute)
func (doc *document) UpdateFromFileOnStorage(libraryRoot string) {
	path := filepath.Join(libraryRoot, doc.localStorage.pathRelativeToLibrary()) //path may be relative if the library root is relative
	statsChanged := doc.localStorage.updateFileStats(path)
	content, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}
	contentChanged := doc.contentMetadata.setFromContent(content)
	if statsChanged || contentChanged {
		doc.updateChangeDate()
	}
}

//Calculates file status using the recorded document path and the library root (relative/absolute)
func (doc *document) CompareToFileOnStorage(libraryRoot string) TrackedFileStatus {
	path := filepath.Join(libraryRoot, doc.localStorage.pathRelativeToLibrary())

	stat, err := os.Stat(path)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			panic(err)
		}
		if doc.removed {
			return RemovedFile
		}
		return MissingFile
	}

	if doc.removed {
		return ZombieFile
	}

	if stat.Size() != doc.contentMetadata.size {
		return ModifiedFile
	}
	//TODO: introduce switch which skips the full read at this point for better performance
	content, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}
	if sha256.Sum256(content) != doc.contentMetadata.sha256Hash {
		return ModifiedFile
	}
	if unixTimestamp(stat.ModTime().Unix()) != doc.localStorage.lastModified {
		//TODO: on performance optimization do a full read if last modified differs
		return TouchedFile
	}
	return UnmodifiedFile
}

func (doc *document) MatchesChecksum(sha256 [sha256.Size]byte) bool {
	return doc.contentMetadata.sha256Hash == sha256
}

func (doc *document) updateChangeDate() {
	doc.changed = unixTimestamp(time.Now().Unix())
}

func (stored *storedFile) setFromRelativePath(relativePath string) {
	stored.directory = SemanticPathFromNative(filepath.Dir(relativePath))
	stored.name = filepath.Base(relativePath)
}

func (stored *storedFile) pathRelativeToLibrary() string {
	return filepath.Join(stored.directory.ToNativeFilepath(), stored.name)
}

func (stored *storedFile) updateFileStats(path string) (hasChanged bool) {
	stat, err := os.Stat(path)
	if err != nil {
		panic(err)
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
	meta.sha256Hash = sha256.Sum256(content)
	if meta.size != oldSize || meta.sha256Hash != oldHash {
		hasChanged = true
	}
	return
}
