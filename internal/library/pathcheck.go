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
		switch status := doc.CompareToFileOnStorage(lib.rootPath, skipReadOnSizeMatch); status {
		case document.UnmodifiedFile:
			result.status = Tracked
		case document.TouchedFile:
			result.status = Touched
			result.referencing = Document{id: doc.Id(), library: lib}
		case document.ModifiedFile:
			result.status = Modified
			result.referencing = Document{id: doc.Id(), library: lib}
		case document.NoFileFound:
			result.status = Missing
			result.referencing = Document{id: doc.Id(), library: lib}
		case document.FileAccessError:
			result.err = fmt.Errorf("could not access last known location (%s) of document %s", doc.AnchoredPath(), doc.Id())
		}
		return
	}

	//path not on active record => inspect

	stat, statErr := os.Stat(absolutePath)
	if statErr != nil {
		switch {
		case !errors.Is(statErr, fs.ErrNotExist):
			result.err = statErr
		case lib.ObsoleteDocumentExistsForPath(absolutePath):
			result.status = Removed //because path does not exist anymore, as expected
		default:
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

	foundMatchingActive := false
	foundModifiedActive := false
	foundMissingActive := false
	foundMatchingObsolete := false

	var anyMissingMatchingActive document.Api
	for _, doc := range lib.documents {
		if doc.MatchesChecksum(fileChecksum) {
			if doc.IsObsolete() {
				foundMatchingObsolete = true
				continue
			}
			statusOfContentMatch := doc.CompareToFileOnStorage(lib.rootPath, skipReadOnSizeMatch)
			switch statusOfContentMatch {
			case document.UnmodifiedFile, document.TouchedFile:
				foundMatchingActive = true
			case document.ModifiedFile:
				foundModifiedActive = true
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
	switch {
	case foundMissingActive:
		result.status = Moved
		result.referencing = Document{id: anyMissingMatchingActive.Id(), library: lib}
	case foundMatchingActive:
		if foundMatchingObsolete {
			result.status = Obsolete
		} else {
			result.status = Duplicate
		}
	case foundModifiedActive:
		if foundMatchingObsolete {
			result.status = Obsolete
		}
	case foundMatchingObsolete:
		result.status = Obsolete
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
