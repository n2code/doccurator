package library

import (
	checksum "crypto/sha256"
	"errors"
	"fmt"
	"github.com/n2code/doccurator/internal/document"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

type CheckedPath struct {
	anchoredPath string //relative to library root, system-native
	status       PathStatus
	referencing  Document
	err          error
}

// CheckFilePath deals with all combinations of the given path being on record and/or [not] existing in reality.
func (lib *library) CheckFilePath(absolutePath string, skipReadOnSizeMatch bool) (result CheckedPath) {
	result.status = Untracked

	defer func() {
		if result.err != nil {
			result.status = Error
		}
	}()

	var inLibrary bool
	result.anchoredPath, inLibrary = lib.getAnchoredPath(absolutePath)
	if !inLibrary {
		result.err = fmt.Errorf("path is not below library root: %s", absolutePath)
		return
	}

	if doc, isOnActiveRecord := lib.activeAnchoredPathIndex[result.anchoredPath]; isOnActiveRecord {
		result.referencing = Document{id: doc.Id(), library: lib}
		switch status := doc.CompareToFileOnStorage(lib.rootPath, skipReadOnSizeMatch); status {
		case document.UnmodifiedFile:
			result.status = Tracked
		case document.TouchedFile:
			result.status = Touched
		case document.ModifiedFile:
			result.status = Modified
		case document.NoFileFound:
			result.status = Missing
		case document.FileAccessError:
			result.err = fmt.Errorf("could not access last known location (%s) of document %s", doc.AnchoredPath(), doc.Id())
		}
		return
	}

	//path not on active record => inspect

	stat, statErr := os.Stat(absolutePath)
	if statErr != nil {
		if !errors.Is(statErr, fs.ErrNotExist) {
			result.err = statErr
		} else if obsoletes := lib.GetObsoleteDocumentsForPath(absolutePath); len(obsoletes) > 0 {
			result.status = Removed                                            //because path does not exist anymore, as expected
			result.referencing = Document{id: obsoletes[0].Id(), library: lib} //TODO: is there a better way than "pick any"?
		} else {
			result.err = fmt.Errorf("path does not exist: %s", absolutePath)
		}
		return
	}
	if stat.IsDir() {
		result.err = fmt.Errorf("path is not a file: %s", absolutePath)
		return
	}
	fileChecksum, checksumErr := calculateFileChecksum(absolutePath)
	if checksumErr != nil {
		result.err = checksumErr
		return
	}

	//file exists that is not on active record, match to known contents by checksum

	foundMatchingNonEmptyActive := false
	foundMissingActive := false
	foundMatchingNonEmptyObsolete := false

	var anyMatchingNonEmptyActive, anyMissingMatchingActive, anyMatchingNonEmptyObsolete document.Api
	for _, doc := range lib.documents {
		if doc.MatchesChecksum(fileChecksum) {
			size, _, _ := doc.RecordedFileProperties()
			if doc.IsObsolete() {
				//empty files are not considered identical to obsoleted empty files unless the path is the same
				if size > 0 || doc.AnchoredPath() == result.anchoredPath {
					foundMatchingNonEmptyObsolete = true
					anyMatchingNonEmptyObsolete = doc
				}
				continue
			}
			statusOfContentMatch := doc.CompareToFileOnStorage(lib.rootPath, skipReadOnSizeMatch)
			switch statusOfContentMatch {
			case document.UnmodifiedFile, document.TouchedFile:
				if size > 0 {
					foundMatchingNonEmptyActive = true
					anyMatchingNonEmptyActive = doc
				}
			case document.ModifiedFile:
				//content has changed so a matching record is moot
			case document.NoFileFound:
				foundMissingActive = true
				anyMissingMatchingActive = doc
			case document.FileAccessError:
				result.err = fmt.Errorf("could not access last known location (%s) of document %s", doc.AnchoredPath(), doc.Id())
				return
			}
		}
	}

	result.status = Untracked
	switch { //the order of cases is significant because it reflects status priority!
	case foundMissingActive:
		result.status = Moved
		result.referencing = Document{id: anyMissingMatchingActive.Id(), library: lib}
	case foundMatchingNonEmptyObsolete:
		result.status = Obsolete
		result.referencing = Document{id: anyMatchingNonEmptyObsolete.Id(), library: lib}
	case foundMatchingNonEmptyActive:
		result.status = Duplicate
		result.referencing = Document{id: anyMatchingNonEmptyActive.Id(), library: lib}
	}

	return
}

func (p CheckedPath) Status() PathStatus {
	return p.status
}

func (p CheckedPath) AnchoredPath() string {
	return p.anchoredPath
}

func (p CheckedPath) GetError() error {
	return p.err
}

func (p CheckedPath) ReferencedDocument() Document {
	return p.referencing
}

func (p CheckedPath) ProbeFile(size *int64, modTime *time.Time, sha256 *[checksum.Size]byte) error {
	absolute := filepath.Join(p.referencing.library.rootPath, p.anchoredPath)

	if size != nil || modTime != nil {
		stat, err := os.Stat(absolute)
		if err != nil {
			return err
		}
		if size != nil {
			*size = stat.Size()
		}
		if modTime != nil {
			*modTime = stat.ModTime()
		}
	}

	if sha256 != nil {
		content, err := os.ReadFile(absolute)
		if err != nil {
			return err
		}
		*sha256 = checksum.Sum256(content)
	}

	return nil
}
