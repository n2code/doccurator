package document

import (
	checksum "crypto/sha256"
	"time"
)

type Id uint64

type TrackedFileStatus rune

const (
	UnmodifiedFile  TrackedFileStatus = iota //file found at expected location and content matches records
	TouchedFile                              //file found at expected location and content matches records but timestamp differs
	ModifiedFile                             //file found at expected location but content differs
	NoFileFound                              //no file at the probed location
	FileAccessError                          //error accessing probed location
)

type Api interface {
	Id() Id
	Recorded() unixTimestamp
	Changed() unixTimestamp
	IsObsolete() bool
	DeclareObsolete()
	Path() string
	SetPath(relativePath string)
	StandardizedFilename() (string, error)
	UpdateFromFileOnStorage(libraryRoot string) (changed bool, err error)
	CompareToFileOnStorage(libraryRoot string) TrackedFileStatus
	MatchesChecksum(sha256 [checksum.Size]byte) bool
	String() string
}

type Index map[Id]Api

func NewDocument(id Id) Api {
	now := unixTimestamp(time.Now().Unix())
	return &document{
		id:       id,
		recorded: now,
		changed:  now,
	}
}
