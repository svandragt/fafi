import datetime

import newspaper

from . import app


def index_site(conn, row):
    url = row[0]
    date_bm_added = row[2]
    d = datetime.datetime.fromtimestamp(date_bm_added / 1000000)
    if any(x in url for x in [".local", ".test"]):
        app.me.AddLogLine("\nS" + url)
        return

    c = conn.cursor()
    c.execute("SELECT url FROM sites WHERE url=?", (url,))
    if c.fetchone():
        app.me.AddLogLine("\n=" + url)
        return "="

    article = newspaper.Article(url)
    try:
        article.download()
        article.parse()
    except newspaper.article.ArticleException:
        app.me.AddLogLine("\nE" + article.download_exception_msg + article.url)
        return "E"

    c.execute(
        "INSERT INTO sites (url, text, date_bm_added) VALUES(?,?,?)",
        (url, article.text, date_bm_added),
    )
    conn.commit()
    app.me.AddLogLine("\n✓" + url + "(" + str(d) + ")")
    return "+"
