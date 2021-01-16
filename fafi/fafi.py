#!/usr/bin/env python3

from contextlib import closing
from subprocess import call
import argparse
import click
import datetime
import hashlib
import newspaper
import os

# fafi
import appdata
import db


def index_site(conn, row, verbose):
    url = row[0]
    date_bm_added = row[2]
    d = datetime.datetime.fromtimestamp(date_bm_added / 1000000)
    if any(x in url for x in [".local", ".test"]):
        print("S", url)
        return

    c = conn.cursor()
    c.execute("SELECT url FROM sites WHERE url=?", (url,))
    if c.fetchone():
        if verbose:
            print("=", url)
        return "="

    article = newspaper.Article(url)
    try:
        article.download()
        article.parse()
    except newspaper.article.ArticleException:
        print("E", article.download_exception_msg, article.url)
        return "E"

    print("✓", url, "(", str(d), ")")
    c.execute(
        "INSERT INTO sites (url, text, date_bm_added) VALUES(?,?,?)",
        (url, article.text, date_bm_added),
    )
    conn.commit()
    return "+"


@click.group()
def cli():
    pass


@click.command("index")
@click.option("-v", "--verbose", is_flag=True, help="Enables verbose mode")
def do_index(verbose):
    bm_db = appdata.get_places_db()
    if bm_db:
        temp_path = appdata.create_temporary_copy(bm_db)

        with db.connect(temp_path) as places:
            with closing(places.cursor()) as ff_cursor:
                ff_cursor = appdata.select_bookmarks(ff_cursor)

                data_path = appdata.data_path()
                with db.connect(data_path) as fafi:
                    db.create_table(fafi)

                    for row in ff_cursor:
                        o = index_site(fafi, row, verbose)
                        if o == "=":
                            continue


@click.command("search")
@click.argument("keywords")
@click.option(
    "--max-results", default=7, show_default=True, help="Return <int> results",
)
def do_search(keywords, max_results):
    print("Searching for:", keywords)
    data_path = appdata.data_path()
    if os.path.exists(data_path):
        with db.connect(data_path) as fafi:
            cursor = db.search(fafi, keywords, max_results)
            if cursor is None:
                print("No results.")
                return

            i = 1
            for row in cursor:
                url = row[0]
                snippet = row[1].replace("\n", " ").strip()
                print(str(i) + ")", url, "\n", snippet, "\n")
                i += 1


cli.add_command(do_index)
cli.add_command(do_search)

if __name__ == "__main__":
    cli()
