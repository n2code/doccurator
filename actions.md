# actions documentation
## `init`
```console
$ doccurator init -h

Usage of init action:
   doccurator [MODE] init [-database=...] [-update-root] DIRECTORY

  Initialize a new library in the given root DIRECTORY. Everything below
  the root is considered to be located 'inside' the library. All files
  inside can be recorded for the purpose of change detection.
  The library state is recorded in a single database file. If either
  the database file or the library folder is moved the library cannot
  be operated on until initialization is repeated with special flags
  for migration.

 Available flags:
  -database string
    	library database file to be created, relative to current working
    	directory unless an absolute path is given
    	(default if empty or flag omitted: "doccurator.db" in DIRECTORY)
  -update-root
    	update existing library file (flag "-database" is mandatory)
    	with new root DIRECTORY instead of creating a fresh library

 Global MODE documentation can be shown by:
    doccurator -h

```
## `status`
```console
$ doccurator status -h

Usage of status action:
   doccurator [MODE] status [FILEPATH...]

  Compare files in the library folder to the records.
  If one or more FILEPATHs are given only compare those files otherwise
  compare all. For an explicit list of paths all states are listed. For
  a full scan (no paths specified) unchanged tracked files are omitted.

 Global MODE documentation can be shown by:
    doccurator -h

```
## `add`
```console
$ doccurator add -h

Usage of add action:
   doccurator [MODE] add [-all-untracked] [-rename] [-force] [-empty] [-abort-on-error] [-auto-id | -id=...] [FILEPATH...]

  Add the file(s) at the given FILEPATH(s) to the library records.
  Interactive mode is launched if no paths are given. The user is prompted to
  decide for each untracked file whether it shall be added and/or renamed.
  Alternatively all untracked files can be added automatically via flag.

 Available flags:
  -abort-on-error
    	abort if any error occurs, do not skip issues (mass operations)
  -all-untracked
    	add all untracked files anywhere inside the library
    	(requires *standardized* filenames or/and flag "-auto-id")
  -auto-id
    	automatically choose free ID based on current time if filename
    	is not *standardized* and hence ID cannot be extracted from it
  -empty
    	allow empty files (only for non-interactive mode with given paths)
  -force
    	allow adding even duplicates, moved, and obsolete files as new
    	(because this is likely undesired and thus blocked by default)
  -id string
    	specify new document ID instead of extracting it from filename
    	(only a single FILEPATH can be given, -all-untracked must not be used)
    	FORMAT 1: doccurator add -id 63835AEV9E my_document.pdf
    	FORMAT 2: doccurator add -id=55565IEV9E my_document.pdf
  -rename
    	rename added files to standardized filename

 Global MODE documentation can be shown by:
    doccurator -h

```
## `update`
```console
$ doccurator update -h

Usage of update action:
   doccurator [MODE] update [FILEPATH...]

  Update the existing library records to match the current state of
  the file(s) at the given FILEPATH(s).

 Global MODE documentation can be shown by:
    doccurator -h

```
## `tidy`
```console
$ doccurator tidy -h

Usage of tidy action:
   doccurator [MODE] tidy [-no-confirm] [-remove-waste-files]

  Interactively do the needful to get the library in sync with the filesystem.
  By default only known records are considered and the filesystem is not touched.

 Available flags:
  -no-confirm
    	suppress prompts and choose defaults ("yes to all")
  -remove-waste-files
    	remove superfluous files with duplicate or obsolete content

 Global MODE documentation can be shown by:
    doccurator -h

```
## `search`
```console
$ doccurator search -h

Usage of search action:
   doccurator [MODE] search ID

  Search for documents with the given ID or substring of an ID

 Global MODE documentation can be shown by:
    doccurator -h

```
## `retire`
```console
$ doccurator retire -h

Usage of retire action:
   doccurator [MODE] retire [FILEPATH...]

  Mark the library records corresponding to the given FILEPATH(s) as
  obsolete. The real file is expected to be removed manually.
  If an identical file appears at a later point in time the library is
  thereby able to recognize it as an obsolete duplicate ("zombie").

 Global MODE documentation can be shown by:
    doccurator -h

```
## `forget`
```console
$ doccurator forget -h

Usage of forget action:
   doccurator [MODE] forget -all-retired | ID...

  Delete the library records corresponding to the given ID(s).
  Only retired documents can be forgotten.

 Available flags:
  -all-retired
    	forget all retired documents

 Global MODE documentation can be shown by:
    doccurator -h

```
## `tree`
```console
$ doccurator tree -h

Usage of tree action:
   doccurator [MODE] tree [-diff] [-here]

  Display the library as a tree which represents the union of all
  library records and the files currently present in the library folder.

 Available flags:
  -diff
    	show only difference to library records, i.e. exclude
    	unchanged tracked and tracked-as-removed files
  -here
    	only print the subtree whose root is the current working directory

 Global MODE documentation can be shown by:
    doccurator -h

```
## `dump`
```console
$ doccurator dump -h

Usage of dump action:
   doccurator [MODE] dump [-exclude-retired]

  Print all library records.

 Available flags:
  -exclude-retired
    	do not print records marked as obsolete ("retired")

 Global MODE documentation can be shown by:
    doccurator -h

```
