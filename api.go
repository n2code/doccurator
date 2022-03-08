package doccurator

import "github.com/n2code/doccurator/internal/document"

// Doccurator lets you interface with a doccurator database whose handle was retrieved using New or Open.
type Doccurator interface {

	// Add creates a single document using a given ID (most likely provided by GetFreeId).
	// Changes need to be committed with PersistChanges.
	Add(id document.Id, path string, allowForDuplicateMovedAndObsolete bool) error

	// AddMultiple creates multiple documents from a set of paths, attempting to use IDs from the filenames.
	// A flag can be set to allow automatic ID generation where no extraction is possible.
	// Attempting to add moved files or files with either duplicate or new content produce errors unless instructed otherwise.
	// Error handling can be toggled to either fail immediately (the library remains clean in that case, i.e. has the same state as before) or ignore issues.
	// Changes need to be committed with PersistChanges.
	AddMultiple(paths []string, allowForDuplicateMovedAndObsolete bool, generateMissingIds bool, abortOnError bool) (added []document.Id, err error)

	// AddAllUntracked collects all untracked files and calls AddMultiple.
	// Flags and error handling behavior are identical to AddMultiple.
	// Changes need to be committed with PersistChanges.
	AddAllUntracked(allowForDuplicateMovedAndObsolete bool, generateMissingIds bool, abortOnError bool) (added []document.Id, err error)

	// UpdateByPath updates the library record corresponding to the given path to match the state of the file.
	// For touched and modified files this updates the file modification timestamp and/or filesize+hash.
	// For moved files this updates the record to reflect the new location as well as time, size, and hash.
	// Attempting to update unmodified tracked files is a no-op, attempts to update files in any other state yields an error.
	// Changes need to be committed with PersistChanges.
	UpdateByPath(path string) error

	// RetireByPath declares the record corresponding to the given path as obsolete.
	// The file is not touched but expected to be deleted. (It does not have to exist.)
	// Attempting to retire an already retired path is a no-op, attempts to retire untracked paths yields an error.
	// Changes need to be committed with PersistChanges.
	RetireByPath(path string) error

	// ForgetById purges a record from the library completely leaving no trace of its past existence.
	// The record needs to be retired already unless the respective flag to do so is set.
	// Changes need to be committed with PersistChanges.
	ForgetById(id document.Id, forceRetire bool) error

	// ForgetAllObsolete purges all retired records (see ForgetById).
	// Changes need to be committed with PersistChanges.
	ForgetAllObsolete()

	// StandardizeLocation renames the file of the given document to conform to the standard format:
	//   file.ext.23456X777.ndoc.ext
	// Library changes need to be committed with PersistChanges.
	// Filesystem changes have an immediate effect but can be reverted by a subsequent call to RollbackAllFilesystemChanges in case of an error.
	StandardizeLocation(id document.Id) error

	// PersistChanges commits all changes to the library database file.
	// The rollback log of RollbackAllFilesystemChanges is emptied.
	PersistChanges() error

	// RollbackAllFilesystemChanges reverts all filesystem changes since the last call to PersistChanges.
	RollbackAllFilesystemChanges() (complete bool)

	// PrintRecord outputs the full state of the given document, uncommitted changes included.
	PrintRecord(id document.Id)

	// PrintAllRecords outputs the full state of all documents in the library, uncommitted changes included.
	PrintAllRecords(excludeRetired bool)

	// PrintTree prints a full filesystem tree of the library root directory.
	// For all files that are not in sync with the library records an indicator is attached to reflect their status with respect to the library.
	PrintTree(excludeUnchanged bool, onlyWorkingDir bool) error

	// PrintStatus compares the given files to the library records and lists all results grouped by status.
	// If no paths are given the full library root directory is scanned recursively and unchanged tracked files are omitted.
	PrintStatus(paths []string) error

	// GetFreeId yields an ID that is not already in use derived from the current time.
	GetFreeId() document.Id

	// SearchByIdPart takes a case-insensitive full/partial ID (non-numeric display format) and compiles
	// a list of all matching record IDs along with their path its current status.
	SearchByIdPart(part string) []SearchResult

	// InteractiveAdd lets the user choose for each untracked file whether to add it, which ID to use, and whether to rename it to match the chosen ID.
	// Library changes need to be committed with a subsequent call to PersistChanges.
	// Filesystem changes (renames) have an immediate effect (and CANNOT be reverted by RollbackAllFilesystemChanges).
	InteractiveAdd(prompt RequestChoice) (cancelled bool)

	// InteractiveTidy guides the user through possible library updates and file system changes:
	// Touched, moved, and modified files can have their records updated.
	// Untracked files with duplicate content (waste) can be deleted.
	// Obsolete files corresponding to retired records (waste) can be deleted.
	// Untracked files with duplicate content (waste) can be deleted.
	// All decisions are up to the user and nothing is changed without confirmation.
	// Library changes need to be committed with a subsequent call to PersistChanges.
	// Filesystem changes have an immediate effect and need to be reverted by RollbackAllFilesystemChanges.
	InteractiveTidy(prompt RequestChoice, removeWaste bool) (cancelled bool)
}

// SearchResult represents a subset of information taken from an existing library record.
type SearchResult struct {
	Id         document.Id
	Path       string //relative to the current working directory
	StatusText string
}

// RequestChoice represents a single-choice decision callback, the first option is considered the default "yes"-like choice.
// If the choice is aborted an empty string must be returned.
// If cleanup is set the implementation is recommended to remove the choice presentation after selection.
type RequestChoice func(request string, options []string, cleanup bool) (choice string)
