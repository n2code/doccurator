package document

type unixTimestamp int64

const missingId DocumentId = 0

type document struct {
	id              DocumentId
	recorded        unixTimestamp   //when the first record of the document entered the system
	changed         unixTimestamp   //when the library record was last changed (change to any field)
	localStorage    storedFile      //either actual or last known physical location
	contentMetadata contentMetadata //last known content information
	removed         bool            //tombstone marker to record removal from library
}

// storedFile is location relative to the storage root
type storedFile struct {
	// directory is a slash separated path relative to the library's root directory
	directory string
	// name is a pure filename without path information
	name         string
	lastModified unixTimestamp
}

type contentMetadata struct {
	size       int64
	sha256Hash [32]byte
}