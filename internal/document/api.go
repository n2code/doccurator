package document

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"time"
)

type DocumentId uint64

type TrackedFileStatus rune

const (
	UnmodifiedFile TrackedFileStatus = iota //file found at expected location and content matches records
	TouchedFile                             //file found at expected location and content matches records but timestamp differs
	ModifiedFile                            //file found at expected location but content differs
	RemovedFile                             //file marked as removed and consequently not found at last known location
	MissingFile                             //file not found at the expected location
	ZombieFile                              //something is present at the file's last known location although file is marked as removed (so it should have been deleted)
)

type DocumentApi interface {
	Id() DocumentId
	SetId(DocumentId)
	Recorded() unixTimestamp
	Changed() unixTimestamp
	Removed() bool
	SetRemoved()
	Path() string
	SetPath(relativePath string)
	UpdateFromFile(location string)
	VerifyRecordedFileStatus() TrackedFileStatus
	MatchesChecksum(sha256 [sha256.Size]byte) bool
	String() string
}

type DocumentIndex map[DocumentId]DocumentApi

func NewDocument(id DocumentId) DocumentApi {
	now := unixTimestamp(time.Now().Unix())
	return &document{
		id:       id,
		recorded: now,
		changed:  now,
	}
}

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

//UpdateFromFile reads and stats the given location so relative/absolute path handling is up
// to the OS and the current working directory. The recorded document path does not matter.
func (doc *document) UpdateFromFile(location string) {
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

//VerifyRecordedFileStatus stats and reads the document's location so the working directory must be
// set to the library root in order to make the access by relative path work.
func (doc *document) VerifyRecordedFileStatus() TrackedFileStatus {
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

func (doc *document) MatchesChecksum(sha256 [sha256.Size]byte) bool {
	return doc.contentMetadata.sha256Hash == sha256
}

func (doc *document) String() string {
	return fmt.Sprintf("Document %s\n  Path:     %s\n  Size:     %d bytes\n  SHA256:   %s\n  Recorded: %s\n  Modified: %s",
		doc.id,
		doc.localStorage.pathRelativeToLibrary(),
		doc.contentMetadata.size,
		hex.EncodeToString(doc.contentMetadata.sha256Hash[:]),
		time.Unix(int64(doc.recorded), 0).Local().Format(time.RFC1123),
		time.Unix(int64(doc.localStorage.lastModified), 0).Local().Format(time.RFC1123))
}
