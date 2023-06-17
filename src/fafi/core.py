import datetime
import time

import newspaper

from fafi import appdata, db


def index_site(url, date_bm_added=None):
    if _is_ignored_url(url):
        print("IGNORED", url)
        return

    with db.connect(appdata.data_path()) as fafi:
        # Skip if the url exists
        cursor = fafi.cursor()
        cursor.execute("SELECT url FROM sites2 WHERE url=?", (url,))
        if cursor.fetchone():
            print("EXISTS", url)
            return

        # Skip errors
        try:
            article = newspaper.Article(url)
            article.download()
            article.parse()
        except newspaper.article.ArticleException:
            print("ERROR", article.download_exception_msg, article.url)
            return

        # Fallback to now
        if date_bm_added is None:
            date_bm_added = time.time()

        d = datetime.datetime.fromtimestamp(date_bm_added / 1000000)

        cursor.execute(
            "INSERT INTO sites2 (title, url, text, date_bm_added) VALUES(?, ?,?,?)",
            (article.title, url, article.text, date_bm_added),
        )
        fafi.commit()
        print("✓", url, "(", str(d), ")")


def _is_ignored_url(url):
    if any(x in url for x in [".local", ".test", "localhost"]):
        return True
    return False
