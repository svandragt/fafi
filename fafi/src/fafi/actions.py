import os

from . import utility, firefox, data, db, index, app



def action_search(keywords, max_results):
    print("Searching for:", keywords)
    data_path = data.data_path()
    if os.path.exists(data_path):
        with db.connect(data_path) as fafi:
            app.me.logbox.clear()
            cursor = db.search(fafi, keywords, max_results)
            rows = list(cursor)

            if len(rows) < 1:
                app.me.AddLogLine("No results.")
            for row in rows:
                url = row[0]
                snippet = row[1].replace("\n", " ").strip()
                app.me.AddLogLine("> " + url + "\n" + snippet + "\n\n")
