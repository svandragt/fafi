import datetime

import newspaper

from . import appdata, db


def index_site(url, date_bm_added=None):
    if _is_ignored_url(url):
        print("\nIGNORED", url)
        return

    with db.connect(appdata.data_path()) as fafi:
        # Skip if the url exists
        cursor = fafi.cursor()
        cursor.execute("SELECT url FROM sites WHERE url=?", (url,))
        if cursor.fetchone():
            print("\nEXISTS", url)
            return

        # Skip errors
        try:
            article = newspaper.Article(url)
            article.download()
            article.parse()
        except newspaper.article.ArticleException:
            print("\nERROR", article.download_exception_msg, article.url)
            return

        d = datetime.datetime.fromtimestamp(date_bm_added / 1000000)
        cursor.execute(
            "INSERT INTO sites (url, text, date_bm_added) VALUES(?,?,?)",
            (url, article.text, date_bm_added),
        )
        fafi.commit()
        print("\n✓", url, "(", str(d), ")")


def _is_ignored_url(url):
    if any(x in url for x in [".local", ".test", "localhost"]):
        return True
    return False
