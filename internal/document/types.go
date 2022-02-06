package document

import "path/filepath"

type unixTimestamp int64

const missingId Id = 0

type document struct {
	id              Id
	recorded        unixTimestamp   //when the first record of the document entered the system
	changed         unixTimestamp   //when the library record was last changed (change to any field)
	localStorage    storedFile      //last known physical location
	contentMetadata contentMetadata //last known content information
	obsolete        bool            //tombstone marker to record removal from library
}

type SemanticPath string //slash-separated regardless of OS

func (p SemanticPath) ToNativeFilepath() string {
	return filepath.FromSlash(string(p))
}

func SemanticPathFromNative(path string) SemanticPath {
	return SemanticPath(filepath.ToSlash(path))
}

// storedFile is location relative to the storage root
type storedFile struct {
	directory    SemanticPath // directory is a semantic path relative to the library's root directory
	name         string       // name is a pure filename without path information
	lastModified unixTimestamp
}

type contentMetadata struct {
	size       int64
	sha256Hash [32]byte
}
