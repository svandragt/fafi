
Fafi
====

Fafi (short for Favorites Finder) is a commandline client to search indexed webpages. Fafi extracts the content of the webpage and stores them into a full-text search database.

Things it does:

* Index single urls, text files containing urls, firefox profiles.
* Incrementally indexing the places database from the Firefox profile folder. (The browser bookmarks) It supports picking a profile from multiple profiles.
* Extract main text content.
* Skips .local, localhost and .test domains.
* Deduplication
* Search results are ranked by relevance and displayed with snippets.

Content extraction courtesy of `Newspaper <https://github.com/codelucas/newspaper>`_.

Users
-----

.. code-block::

   pipx install fafi
   fafi --help
   fafi index --firefox
   fafi index --url=https://mylink
   fafi index --list=bookmarks.html
   fafi search 'linux'

Developers
----------

.. code-block::

   # Install project requirements.
   poetry install

   # Help
   poetry run fafi --help
 

.. image:: https://github.com/svandragt/fafi/assets/594871/d70bd34c-009c-4ead-9083-1ddc9a88ec29
   :target: https://github.com/svandragt/fafi/assets/594871/d70bd34c-009c-4ead-9083-1ddc9a88ec29
   :alt: TUI results for search query "linux"

