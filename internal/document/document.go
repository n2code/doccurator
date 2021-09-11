package document

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

type PathStatus rune

const (
	UntrackedDocument  PathStatus = '+'
	ModifiedDocument   PathStatus = '!'
	MovedDocument      PathStatus = '>'
	UnmodifiedDocument PathStatus = '='
	RemovedDocument    PathStatus = 'X'
	MissingDocument    PathStatus = '?'
)

type TrackedFileStatus rune

const (
	UnmodifiedFile TrackedFileStatus = iota //file found at expected location and content matches records
	TouchedFile                             //file found at expected location and content matches records but timestamp differs
	ModifiedFile                            //file found at expected location but content differs
	RemovedFile                             //file marked as removed and consequently not found at last known location
	MissingFile                             //file not found at the expected location
	ZombieFile                              //something is present at the file's last known location although file is marked as removed (so it should have been deleted)
)

func NewDocument(id DocumentId) *Document {
	now := unixTimestamp(time.Now().Unix())
	return &Document{
		id:       id,
		recorded: now,
		changed:  now,
	}
}

func (doc *Document) Id() DocumentId {
	return doc.id
}

//SetId allows ID restoration after deserialization and thus does not affect the change timestamp
func (doc *Document) SetId(id DocumentId) {
	if doc.id != MissingId {
		panic("ID change attempt")
	}
	doc.id = id
}

func (doc *Document) Recorded() unixTimestamp {
	return doc.recorded
}

func (doc *Document) Changed() unixTimestamp {
	return doc.changed
}

func (doc *Document) Removed() bool {
	return doc.removed
}

func (doc *Document) SetRemoved() {
	doc.removed = true
}

//Path returns a filepath relative to the library root directory
func (doc *Document) Path() string {
	return doc.localStorage.pathRelativeToLibrary()
}

//SetPath expects a filepath relative to the library root directory
func (doc *Document) SetPath(relativePath string) {
	doc.localStorage.setFromRelativePath(relativePath)
	doc.updateChangeDate()
}

func (doc *Document) updateChangeDate() {
	doc.changed = unixTimestamp(time.Now().Unix())
}

//UpdateFromFile reads and stats the given location so relative/absolute path handling is up
// to the OS and the current working directory. The recorded document path does not matter.
func (doc *Document) UpdateFromFile(location string) {
	statsChanged := doc.localStorage.updateFileStats(location)
	content, err := os.ReadFile(location)
	if err != nil {
		panic(err)
	}
	contentChanged := doc.contentMetadata.setFromContent(content)
	if statsChanged || contentChanged {
		doc.updateChangeDate()
	}
}

func (stored *storedFile) setFromRelativePath(relativePath string) {
	stored.directory = filepath.ToSlash(filepath.Dir(relativePath))
	stored.name = filepath.Base(relativePath)
}

func (stored *storedFile) pathRelativeToLibrary() string {
	return filepath.Join(filepath.FromSlash(stored.directory), stored.name)
}

func (stored *storedFile) updateFileStats(location string) (hasChanged bool) {
	stat, err := os.Stat(location)
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

//VerifyRecordedFileStatus stats and reads the document's location so the working directory must be
// set to the library root in order to make the access by relative path work.
func (doc *Document) VerifyRecordedFileStatus() TrackedFileStatus {
	location := doc.localStorage.pathRelativeToLibrary()

	stat, err := os.Stat(location)
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
	content, err := os.ReadFile(location)
	if err != nil {
		panic(err)
	}
	if sha256.Sum256(content) != doc.contentMetadata.sha256Hash {
		return ModifiedFile
	}
	if unixTimestamp(stat.ModTime().Unix()) != doc.localStorage.lastModified {
		return TouchedFile
	}
	return UnmodifiedFile
}

func (doc *Document) String() string {
	return fmt.Sprintf("Document %d\n  Path:     %s\n  Size:     %d bytes\n  SHA256:   %s\n  Recorded: %s\n  Modified: %s",
		doc.id,
		doc.localStorage.pathRelativeToLibrary(),
		doc.contentMetadata.size,
		hex.EncodeToString(doc.contentMetadata.sha256Hash[:]),
		time.Unix(int64(doc.recorded), 0).Local().Format(time.RFC1123),
		time.Unix(int64(doc.localStorage.lastModified), 0).Local().Format(time.RFC1123))
}
