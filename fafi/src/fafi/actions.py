import os
from contextlib import closing

from . import utility, firefox, data, db,  index, app


def action_index_with_db(places_db):
    temp_path = utility.create_temporary_copy(places_db)
    with db.connect(temp_path) as places:
        with closing(places.cursor()) as ff_cursor:
            ff_cursor = firefox.select_bookmarks(ff_cursor)

            data_path = data.data_path()
            with db.connect(data_path) as fafi:
                db.create_table(fafi)

                o = None

                for row in ff_cursor:
                    o = index.index_site(fafi, row)
                    if o == "=":
                        continue

                if not o:
                    print('\nNothing to index.')
                    app.me.AddLogLine('Nothing to index.')


def action_search(keywords, max_results):
    print("Searching for:", keywords)
    data_path = data.data_path()
    if os.path.exists(data_path):
        with db.connect(data_path) as fafi:
            app.me.logbox.clear()
            cursor = db.search(fafi, keywords, max_results)
            if cursor is None:
                app.me.AddLogLine("No results.")
                return

            i = 1
            for row in cursor:
                url = row[0]
                snippet = row[1].replace("\n", " ").strip()
                app.me.AddLogLine("> ", url, "\n", snippet, "\n\n")
                i += 1
