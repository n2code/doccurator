# doccurator
> *lightweight document indexing, modification tracking, and duplicate detection*

This command-line tool helps you to keep track of a collection of documents by persisting
their checksums, filenames, and modification timestamps.
```console
$ doccurator status

 Missing (2 files)
  [?] bills/electricity_2021.pdf.2552856HTD.ndoc.pdf
  [?] certificates/supercyberhacker-cert.crt.33484C4OTD.ndoc.crt

 Untracked (2 files)
  [+] bills/water_2021.pdf.85484P4OTD.ndoc.pdf
  [+] backup_mails.zip

 Moved (1 file)
  [>] bills/FriendlyNeighborhoodWaterworks_YearlyBill_2020.pdf.33484C4OTD.ndoc.pdf

 Modified (1 file)
  [!] passwords.kdbx

```

Right now the feature scope is rather minimal. It focuses on redundantly storing the file metadata 
and hence detecting and displaying changes in a given directory tree. Duplicates (with respect to 
file content) can be detected and therefore safely discarded. 
A [robust alphanumeric identifier](https://github.com/n2code/ndocid) is 
assigned to each recorded file which can be mirrored in the file name for global identification.

Doccurator operates just like git in the sense that a *library* denotes a certain directory whose 
contents (focus on files only) are being tracked. All changes have to be manually approved, new 
files have to be explicitly added, deletions have to be manually confirmed and so on. Except for 
a few special flags which explicitly demand file system changes no content is ever touched - only 
read. There is no staging area / index for simplicity reasons. Doccurator commands take filenames 
as relative arguments and detect automatically which library (root folder) they're operating in.

## Usage
```console
$ doccurator -h

Usage:
   doccurator [-v|-q] [-t] [-a] [-p] [-h] <ACTION> [FLAG] [TARGET]

 ACTIONs:  init  status  add  update  tidy  search  retire  forget  tree  dump

  -a	Do not skip anything during recursive scans (all mode):
    	  Unless flag is set the library database file is skipped.
    	  Files/folders starting with "." are not considered either.
    	  The function of ignore files is not affected.
  -h	Display general usage help
  -p	Do not use terminal escape sequence features such as colors (plain mode)
  -q	Output as little as possible, i.e. only requested information (quiet mode)
  -t	Do not apply optimizations (thorough mode), for example:
    	  Unless flag is set files with unchanged modification time are not read.
  -v	Output more details on what is done (verbose mode)

 FLAG(s) and TARGET(s) are action-specific.
 You can read the help on any action verb:
    doccurator <ACTION> -h

```
Check [actions.md](actions.md) for the full action verb help.
