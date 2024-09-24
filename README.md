# fafi
Web content indexing and search tool.

1. Easily index your browser bookmarks: importing list of files, a live Firefox profile, and individual URLs
2. Store the readability content of each bookmark
3. Full text search matching the title and contents of the collection.

On the latest incarnation:

* Aim of the rewrite in GO is to have a single binary that starts a webserver which can be started via systemd and queried via firefox.
* On launch it will index any missing bookmarks (in a background process)

This is how it looks:

![image](https://github.com/svandragt/fafi/assets/594871/c7f9a06c-fa2a-430b-a9be-2bc66b0615d3)


## Environment variables

```env
# Defaults are below:

# Port number for the webserver.
FAFI_PORT=8080
# Set to non-empty value to skip populating the database with sample records.
FAFI_SKIP_RECORDS=0
# Set to 0 to disable indexing on startup
FAFI_ENABLE_INDEXING=1
# Default database path:
FAFI_DB_FILEPATH=~/fafi.sqlite3

# Enable importing bookmarks from Firefox profile db:
FAFI_FIREFOX=/home/san.../32kuswpy.default-release/places.sqlite

```

## Command-line arguments

Each of the environment variables are available as a longform command-line argument by discarding `FAFI_` and lower-casing the result, replacing underscores with dashes. For example `FAFI_ENABLE_INDEXING=0` and `--enable-indexing=0` are equivalent.


## Build and run

```shell
# build with full-text search
$ go build --tags fts5 -o tmp/fafi2 fafi2 
# run, eg indexing your Firefox places database
$ tmp/fafi2 --firefox=/path/to/firefox/profile/places.sqlite
```
