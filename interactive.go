package doccurator

import (
	"fmt"
	"github.com/n2code/doccurator/internal/library"
	"github.com/n2code/doccurator/internal/output"
	"os"
	"path/filepath"
	"strings"
)

const choiceAborted = ""

func (d *doccurator) InteractiveTidy(choice RequestChoice, removeWaste bool) (cancelled bool) {
	fmt.Fprint(d.verboseOut, "Tidying up library...\n")
	paths, _ := d.appLib.Scan(func(absoluteFilePath string) bool {
		return false
	}, d.optimizedFsAccess)

	buckets := make(map[library.PathStatus][]*library.CheckedPath)
	for i, path := range paths {
		buckets[path.Status()] = append(buckets[path.Status()], &paths[i])
	}

	var deletionCommitQueue []func() error

	for _, status := range []library.PathStatus{library.Touched, library.Moved, library.Modified, library.Obsolete, library.Duplicate} {
		count := len(buckets[status])
		if count == 0 {
			continue
		}
		var declarationSingle, declarationMultiple, promptMassProcessing, question, subject, pastParticiple string
		switch status {
		case library.Touched, library.Moved, library.Modified:
			declarationSingle = "1 document on record has its file %s.\n"         // touched/moved/modified
			declarationMultiple = "%d documents on record have their files %s.\n" // <count> + touched/moved/modified
			promptMassProcessing = "Update %s records?"                           // touched/moved/modified
			question = "Update record?"
			subject = "document"
			pastParticiple = "updated"
		case library.Obsolete, library.Duplicate:
			if !removeWaste {
				continue
			}
			declarationSingle = "1 file present has %s content.\n"      // duplicate/obsolete
			declarationMultiple = "%d files present have %s content.\n" // <count> + duplicate/obsolete
			promptMassProcessing = "Delete %s files?"                   // duplicate/obsolete
			question = "Delete file?"
			subject = "file"
			pastParticiple = "marked for deletion"
		}

		upperStatus := strings.ToUpper(status.String())
		lowerStatus := strings.ToLower(status.String())
		var doChange, decideIndividually bool
		{
			fmt.Fprintf(d.extraOut, "\n")
			if count == 1 {
				fmt.Fprintf(d.verboseOut, declarationSingle, upperStatus)
				decideIndividually = true
			} else {
				fmt.Fprintf(d.verboseOut, declarationMultiple, count, upperStatus)
				switch choice(fmt.Sprintf(promptMassProcessing, lowerStatus), []string{"All", "Decide individually", "Skip"}, true) {
				case "All":
					decideIndividually = false
					doChange = true
				case "Decide individually":
					decideIndividually = true
				case "Skip":
					decideIndividually = false
					doChange = false
				case choiceAborted:
					return true
				}
			}
		}

		changeCount := 0
	NextChange:
		for _, path := range buckets[status] {
			absolute := filepath.Join(d.appLib.GetRoot(), path.AnchoredPath())
			displayPath := d.displayablePath(absolute, true, false)

			if decideIndividually {
				switch choice(fmt.Sprintf("%s [%s] - %s", displayPath, lowerStatus, question), []string{"Yes", "No"}, true) {
				case "Yes":
					doChange = true
				case "No":
					doChange = false
				case choiceAborted:
					return true
				}
			}

			if doc := path.ReferencedDocument(); doChange {
				switch status {
				case library.Moved:
					err := d.appLib.SetDocumentPath(doc, absolute)
					if err != nil {
						fmt.Fprintf(d.errOut, "update failed (%s): %s\n", displayPath, err)
						continue NextChange
					}
					fallthrough
				case library.Touched, library.Modified:
					_, err := d.appLib.UpdateDocumentFromFile(doc)
					if err != nil {
						fmt.Fprintf(d.errOut, "update failed (%s): %s\n", displayPath, err)
						continue NextChange
					} else {
						fmt.Fprintf(d.extraOut, "%s [%s] - Updated %s.\n", displayPath, lowerStatus, doc.Id())
					}
				case library.Obsolete, library.Duplicate:
					tempDir, err := os.MkdirTemp("", "doccurator-tidy-delete-staging-*")
					if err != nil {
						fmt.Fprintf(d.errOut, "deletion preparation failed (%s): %s\n", displayPath, err)
						continue NextChange
					}
					deleteStagingDir := func(stagingDir string) func() error {
						return func() error {
							if err := os.RemoveAll(stagingDir); err != nil {
								return fmt.Errorf("could not delete temporary staging directory (%s): %w", stagingDir, err)
							}
							return nil
						}
					}(tempDir)

					backup := filepath.Join(tempDir, filepath.Base(absolute))
					if err := os.Rename(absolute, backup); err != nil {
						fmt.Fprintf(d.errOut, "deletion failed (%s): %s\n", displayPath, err)
						if err := deleteStagingDir(); err != nil {
							fmt.Fprintf(d.errOut, "%s\n", err)
						}
						continue NextChange
					}
					deletionCommitQueue = append(deletionCommitQueue, deleteStagingDir)

					d.rollbackLog = append(d.rollbackLog, func(source string, target string, stagingDir string) func() error {
						return func() error {
							if err := os.Rename(source, target); err != nil {
								return err
							}
							return os.RemoveAll(stagingDir)
						}
					}(backup, absolute, tempDir))

					fmt.Fprintf(d.extraOut, "%s [%s] - Marked for delete.\n", displayPath, lowerStatus)
				}
				changeCount++
			} else {
				fmt.Fprintf(d.extraOut, "%s - Skipped.\n", displayPath)
			}
		}
		fmt.Fprintf(d.extraOut, "%d %s %s %s.\n", changeCount, upperStatus, output.Plural(changeCount, subject, subject+"s"), pastParticiple)
	}

	fmt.Fprint(d.extraOut, "\n")

	if len(deletionCommitQueue) > 0 {
		fmt.Fprintf(d.extraOut, "Committing deletions...\n")
		for _, commitDelete := range deletionCommitQueue {
			if err := commitDelete(); err != nil {
				//errors are reported but do not constitute an overall failure as a rollback would not work and removal from the original directory is already complete by now
				// => failure is only possible theoretically anyway as the application should be able to remove the staging directory it has just created
				fmt.Fprintf(d.errOut, "deletion has leftovers: %s\n", err)
			}
		}
	}

	fmt.Fprint(d.verboseOut, "Tidy operation complete.\n")
	return false
}

func (d *doccurator) InteractiveAdd(choice RequestChoice) (cancelled bool) {
	results, _ := d.appLib.Scan(d.isLibFilePath, true) //read can be skipped because it does not affect correct detection of "untracked" status

	doRename, skipRenameChoice := false, false
NextCandidate:
	for _, checked := range results {
		switch checked.Status() {
		case library.Untracked:
			absolute := filepath.Join(d.appLib.GetRoot(), checked.AnchoredPath())
			displayPath := d.displayablePath(absolute, true, false)

			usingExtractedId := true
			extractedId, idErr := ExtractIdFromStandardizedFilename(absolute)
			hasExtractedId := idErr == nil
			newId := extractedId
			if !hasExtractedId {
				newId = d.GetFreeId()
				usingExtractedId = false
			}

			for decided := false; !decided; {
				if usingExtractedId {
					switch choice(fmt.Sprintf("Add %s using ID from filename? [%s]", displayPath, extractedId), []string{"Yes", "New ID", "Skip"}, true) {
					case "Yes":
						decided = true
					case "New ID":
						newId = d.GetFreeId()
						usingExtractedId = false
					case "Skip":
						continue NextCandidate
					case choiceAborted:
						return true
					}
				} else {
					options := []string{"Yes"}
					if hasExtractedId {
						options = append(options, "From filename")
					}
					options = append(options, "Skip")
					switch choice(fmt.Sprintf("Add %s using new generated ID? [%s]", displayPath, newId), options, true) {
					case "Yes":
						decided = true
					case "From filename":
						newId = extractedId
						usingExtractedId = true
					case "Skip":
						continue NextCandidate
					case choiceAborted:
						return true
					}
				}
			}

			var newDoc library.Document
			var addErr error
			if newDoc, addErr = d.addSingle(newId, absolute, false); addErr != nil {
				fmt.Fprintf(d.errOut, "Adding failed (%s): %s\n", displayPath, addErr)
				continue NextCandidate
			}

			differentNewName, namePreviewErr, _ := newDoc.RenameToStandardNameFormat(true)
			if namePreviewErr != nil {
				fmt.Fprintf(d.errOut, "Skipping rename for new document [%s]: %s\n", newId, namePreviewErr)
				continue NextCandidate
			}
			if differentNewName == "" { //nothing to rename
				continue NextCandidate
			}

			if !skipRenameChoice {
				idChange := "include"
				if hasExtractedId {
					idChange = "update"
				}
				question := fmt.Sprintf(" ... rename to %s to %s ID in filename?", differentNewName, idChange)
				switch choice(question, []string{"Rename once", "Always rename", "Never rename", "Keep filename"}, true) {
				case "Always rename":
					skipRenameChoice = true
					fallthrough
				case "Rename once":
					doRename = true
				case "Never rename":
					skipRenameChoice = true
					fallthrough
				case "Keep filename":
					doRename = false
				case choiceAborted:
					return true
				}
			}

			if !doRename {
				continue NextCandidate
			}

			if _, renameErr, _ := newDoc.RenameToStandardNameFormat(false); renameErr != nil {
				fmt.Fprintf(d.errOut, "Skipping rename of %s: %s\n", newId, namePreviewErr)
				continue NextCandidate
			}
			fmt.Fprintf(d.extraOut, "  => Renamed to: %s\n", differentNewName)

		case library.Error:
			fmt.Fprintf(d.extraOut, "Skipping uncheckable (%s): %s\n", checked.AnchoredPath(), checked.GetError())
		}
	}
	return false
}
