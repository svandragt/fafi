#!/usr/bin/env python3

from contextlib import closing
from subprocess import call
import argparse
import click
import hashlib
import newspaper
import os

# fafi
import appdata
import db


def index_site(conn, row, verbose):
    url = row[0]
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

    print("✓", url)
    c.execute("INSERT INTO sites (url, text) VALUES(?,?)", (url, article.text))
    conn.commit()
    return "+"


@click.group()
def cli():
    pass


@click.command("index")
@click.option(
    "--max-exists",
    default=10,
    show_default=True,
    help="Stop after <int> sites have been indexed sequentially.",
)
@click.option("-v", "--verbose", is_flag=True, help="Enables verbose mode")
def do_index(verbose, max_exists):
    bm_db = appdata.get_bookmarks_db()
    exists = 0
    if bm_db:
        temp_path = appdata.create_temporary_copy(bm_db)

        with db.create_connection(temp_path) as places:
            with closing(places.cursor()) as ff_cursor:
                ff_cursor = appdata.select_bookmarks(ff_cursor)

                db_path = appdata.db_path()
                with db.create_connection(db_path) as fafi:
                    db.create_table(fafi)

                    for row in ff_cursor:
                        o = index_site(fafi, row, verbose)
                        if o == "=":
                            exists += 1
                            if max_exists != -1 and exists >= max_exists:
                                return
                            continue
                        # Reset on error or new index
                        exists = 0


@click.command("search")
@click.argument("query")
@click.option(
    "--max-results", default=7, show_default=True, help="Return <int> results",
)
def do_search(query, max_results):
    print("Searching for:", query)
    db_path = appdata.db_path()
    if os.path.exists(db_path):
        with db.create_connection(db_path) as fafi:
            cursor = db.search(fafi, query, max_results)
            if cursor is None:
                print("No results.")
                return

            i = 1
            for row in cursor:
                print(
                    str(i) + ")", row[0], "\n", row[1].replace("\n", " ").strip(), "\n"
                )
                i += 1


cli.add_command(do_index)
cli.add_command(do_search)

if __name__ == "__main__":
    cli()
