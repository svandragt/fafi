from contextlib import closing

import appdata
import datetime
import db
import newspaper


def index_site(conn, row, verbose):
    url = row[0]
    date_bm_added = row[2]
    d = datetime.datetime.fromtimestamp(date_bm_added / 1000000)
    if any(x in url for x in [".local", ".test"]):
        print("\nS", url)
        return

    c = conn.cursor()
    c.execute("SELECT url FROM sites WHERE url=?", (url,))
    if c.fetchone():
        if verbose:
            print("\n=", url)
        return "="

    article = newspaper.Article(url)
    try:
        article.download()
        article.parse()
    except newspaper.article.ArticleException:
        print("\nE", article.download_exception_msg, article.url)
        return "E"

    print("\n✓", url, "(", str(d), ")")
    c.execute(
        "INSERT INTO sites (url, text, date_bm_added) VALUES(?,?,?)",
        (url, article.text, date_bm_added),
    )
    conn.commit()
    return "+"


def index_with_db(places_db, verbose):
    temp_path = appdata.create_temporary_copy(places_db)
    with db.connect(temp_path) as places:
        with closing(places.cursor()) as ff_cursor:
            ff_cursor = appdata.select_bookmarks(ff_cursor)

            data_path = appdata.data_path()
            with db.connect(data_path) as fafi:
                db.create_table(fafi)

                o = None

                for row in ff_cursor:
                    o = index_site(fafi, row, verbose)
                    if o == "=":
                        continue

                if not o:
                    print('\nNothing to index.')
