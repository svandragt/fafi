# fafi
Web content indexing and search tool

* Aim of the rewrite in GO is to have a single binary that starts a webserver which can be started via systemd and queried via firefox.
* On launch it will index any missing bookmarks (in a background process)

## Environment variables

```env
# Defaults are below:

# Port number
FAFI_PORT=8080
# Set to non-empty value to skip populating the database with sample records.
FAFI_SKIP_RECORDS=
# Set to 0 to disable indexing
FAFI_ENABLE_INDEXING=1
```
