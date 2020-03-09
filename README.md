# FaFF
Search Firefox bookmark contents, with this commandline client. FaFF extracts the content of the bookmarks and stores them into a searchable SQLite database.

WIP:

 * Detects your places database from the Firefox profile folder.
 * Extract main text content from all bookmarks into `./data/faff.sqlite`.
 * Skips .local domains
 * Skips pages that are already indexed.

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
> Indexing:  ~/Library/Application Support/Firefox/Profiles/31bwvkr0.dev-edition-default-1583536459682/places.sqlite
> = https://github.com/humanmade/Cavalcade-Runner/blob/master/inc/class-runner.php#L236-L239
> = https://gist.github.com/svandragt/99274f1f486b55964407d8c5d26a8c64
> ...

# Search for VPN
./faff.py search vpn

> Searching for: vpn
> 1) https://firejail.wordpress.com/
> 2) https://blog.elementary.io/introducing-elementary-os-5-1-hera/
> 3) https://arstechnica.com/gadgets/2019/12/nebula-vpn-routes-between-hosts-privately-flexibly-and-efficiently/
```

