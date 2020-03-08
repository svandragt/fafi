# faff
Favourites Firefox indexing and search tool.

WIP:

 * Extract main text content from all bookmarks into ./data/*.txt files
 * Skips .local domains

It does not return the URLs yet, although they're the first line in the text file.

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
