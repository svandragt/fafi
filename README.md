# FaFF
Search Firefox bookmark contents, with this commandline client. FaFF extracts the content of the bookmarks and stores them into a searchable SQLite database.

Things it does:

 * Detects your places database from the Firefox profile folder.
 * Extract main text content from all bookmarks into `./data/faff.sqlite`.
 * Skips .local domains
 * Skips pages that are already indexed.
 * Search results are ranked by relevance and displayed with snippets.

URLs are stored together with the main page context as determined by [Newspaper](https://github.com/codelucas/newspaper).

```
# Install project requirements.
pipenv install

# Log in to a python shell
pipenv shell

# Make faff executable
chmod +x faff.py

# Index bookmarks
./faff.py index

# Search for linux
./faff.py search 'linux'
```
![search query](https://user-images.githubusercontent.com/594871/76201330-ffcba880-61ea-11ea-9fdd-cc32a90deecd.png)
