package document

import (
	"crypto/sha256"
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
	AccessError                             //error accessing probed location
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
	UpdateFromFileOnStorage(libraryRoot string)
	CompareToFileOnStorage(libraryRoot string) TrackedFileStatus
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
