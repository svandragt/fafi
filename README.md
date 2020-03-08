# FaFF
Search Firefox bookmark contents, with this commandline client. FaFF indexes the main content of the pages into a plain text database and allows you to use linux tools to search through them.

WIP:

 * Extract main text content from all bookmarks into ./data/*.txt files
 * Skips .local domains
 * Skips pages that are already indexed.

URLs are stored in the first line of the text.

```
# Setup data directory.
mkdir data

# Install project requirements.
pipenv install

# Update path to your places.sqlite location.
nano faff.py

# Index bookmarks 
pipenv run python ./faff.py

# Search for bookmarks containing a query such as 'vpn'
grep -iR 'vpn' data
```

![output](https://user-images.githubusercontent.com/594871/76162502-4ad7b400-6136-11ea-824a-72ccda1cada7.png)
