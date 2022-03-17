package doccurator

import (
	"fmt"
	"github.com/n2code/doccurator/internal/library"
	out "github.com/n2code/doccurator/internal/output"
	"os"
	"path/filepath"
	"strings"
)

const choiceAborted = ""

func (d *doccurator) InteractiveTidy(choice RequestChoice, removeWaste bool) (cancelled bool) {
	d.Print(out.Verbose, "Tidying up library...\n")

	// this scan has no skip conditions because consciously added content shall be treated as such
	// => status untracked & missing not relevant in tidy operation and the filter is usually used to prevent accidental adding or noise in queries
	paths, _ := d.appLib.Scan(nil, nil, d.optimizedFsAccess)

	buckets := make(map[library.PathStatus][]*library.CheckedPath)
	for i, path := range paths {
		buckets[path.Status()] = append(buckets[path.Status()], &paths[i])
	}

	var deletionCommitQueue []func() error
	coloredError := func(err error) string {
		return d.printer.Sprintf("%s%s%s%s", library.ColorForStatus(library.Error), out.Invert, err, out.Reset)
	}

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
		coloredStatus := func(text string) string {
			return d.printer.Sprintf("%s%s%s", library.ColorForStatus(status), text, out.DefaultForeground)
		}
		var doChange, decideIndividually bool
		{
			d.Print(out.Normal, "\n")
			if count == 1 {
				d.Print(out.Verbose, declarationSingle, coloredStatus(upperStatus))
				decideIndividually = true
			} else {
				d.Print(out.Verbose, declarationMultiple, count, coloredStatus(upperStatus))
				switch choice(fmt.Sprintf(promptMassProcessing, coloredStatus(lowerStatus)), []string{"All", "Decide individually", "Skip"}, true) {
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
				switch choice(d.printer.Sprintf("%s%s%s%s [%s]%s - %s", library.ColorForStatus(status), out.BoldIntensity, displayPath, out.NormalIntensity, lowerStatus, out.DefaultForeground, question), []string{"Yes", "No"}, true) {
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
						d.Print(out.Error, "update failed (%s): %s\n", displayPath, coloredError(err))
						continue NextChange
					}
					fallthrough
				case library.Touched, library.Modified:
					_, err := d.appLib.UpdateDocumentFromFile(doc)
					if err != nil {
						d.Print(out.Error, "update failed (%s): %s\n", displayPath, coloredError(err))
						continue NextChange
					} else {
						d.Print(out.Normal, "%s [%s] - Updated %s.\n", displayPath, lowerStatus, doc.Id())
					}
				case library.Obsolete, library.Duplicate:
					tempDir, err := os.MkdirTemp("", "doccurator-tidy-delete-staging-*")
					if err != nil {
						d.Print(out.Error, "deletion preparation failed (%s): %s\n", displayPath, coloredError(err))
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
						d.Print(out.Error, "deletion failed (%s): %s\n", displayPath, coloredError(err))
						if err := deleteStagingDir(); err != nil {
							d.Print(out.Error, "%s\n", coloredError(err))
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

					d.Print(out.Normal, "%s [%s] - Marked for delete.\n", displayPath, lowerStatus)
				}
				changeCount++
			} else {
				d.Print(out.Normal, "%s - Skipped.\n", displayPath)
			}
		}
		d.Print(out.Normal, "%s%d %s %s %s.%s\n", out.FaintIntensity, changeCount, lowerStatus, out.Plural(changeCount, subject, subject+"s"), pastParticiple, out.Reset)
	}

	d.Print(out.Normal, "\n")

	if len(deletionCommitQueue) > 0 {
		d.Print(out.Normal, "Committing deletions...\n")
		for _, commitDelete := range deletionCommitQueue {
			if err := commitDelete(); err != nil {
				//errors are reported but do not constitute an overall failure as a rollback would not work and removal from the original directory is already complete by now
				// => failure is only possible theoretically anyway as the application should be able to remove the staging directory it has just created
				d.Print(out.Error, "deletion has leftovers: %s\n", coloredError(err))
			}
		}
	}

	d.Print(out.Verbose, "Tidy operation complete.\n")
	return false
}

func (d *doccurator) InteractiveAdd(choice RequestChoice) (cancelled bool) {
	results, _ := d.appLib.Scan([]library.PathSkipEvaluator{d.isLibFile}, nil, true) //read can be skipped because it does not affect correct detection of "untracked" status

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
				d.Print(out.Error, "Adding failed (%s): %s\n", displayPath, addErr)
				continue NextCandidate
			}

			differentNewName, namePreviewErr, _ := newDoc.RenameToStandardNameFormat(true)
			if namePreviewErr != nil {
				d.Print(out.Error, "Skipping rename for new document [%s]: %s\n", newId, namePreviewErr)
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
				d.Print(out.Error, "Skipping rename of %s: %s\n", newId, namePreviewErr)
				continue NextCandidate
			}
			d.Print(out.Normal, "  => Renamed to: %s\n", differentNewName)

		case library.Error:
			d.Print(out.Normal, "Skipping uncheckable (%s): %s\n", checked.AnchoredPath(), checked.GetError())
		}
	}
	return false
}
