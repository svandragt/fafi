
Fafi
====

Search Firefox bookmark contents, with this commandline client. Fafi extracts the content of the bookmarks and stores them into a searchable SQLite database.

Things it does:


* Detects your places database from the Firefox profile folder. (support for picking a profile from multiple profiles)
* Extract main text content from all bookmarks into ``<user_data_dir>/fafi/data.sqlite``.
* Skips .local and .test domains.
* Skips pages that are already indexed.
* Search results are ranked by relevance and displayed with snippets.

URLs are stored together with the main page context as determined by `Newspaper <https://github.com/codelucas/newspaper>`_.

Users
-----

.. code-block::

   pipx install fafi
   fafi --help
   fafi index
   fafi search 'linux'

Developers
----------

.. code-block::

   # Install project requirements.
   poetry install

   # Log in to a python shell
   poetry shell

   # Make faff executable
   chmod +x fafi.py

   # Help on commands
   ./fafi.py --help
   
   # Index bookmarks
   ./fafi.py index

   # Search for linux
   ./fafi.py search 'linux'


.. image:: https://user-images.githubusercontent.com/594871/76201330-ffcba880-61ea-11ea-9fdd-cc32a90deecd.png
   :target: https://user-images.githubusercontent.com/594871/76201330-ffcba880-61ea-11ea-9fdd-cc32a90deecd.png
   :alt: search query

