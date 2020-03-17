#!/usr/bin/env python3

from contextlib import closing
from subprocess import call
import appdirs
import argparse
import click
import hashlib
import newspaper
import os
import shutil
import sqlite3
import tempfile


fafi = None

# execute a query on sqlite cursor
def execute_query(cursor, query):
    try:
        cursor.execute(query)
    except Exception as error:
        print(str(error) + "\n " + query)


# get bookmarks from firefox sqlite database file and print all
def select_bookmarks(cursor):
    bookmarks_query = """
    SELECT DISTINCT
        url, moz_places.title from moz_places  
    JOIN 
        moz_bookmarks on moz_bookmarks.fk=moz_places.id 
    WHERE 
        moz_places.url like 'http%'
    ORDER BY 
        dateAdded desc
    """
    execute_query(cursor, bookmarks_query)
    return cursor


def create_temporary_copy(path):
    temp_dir = tempfile.gettempdir()
    temp_path = os.path.join(temp_dir, "temp_file_name")
    shutil.copy2(path, temp_path)
    return temp_path


def get_bookmarks_path():
    # set the path of firefox folder with databases
    bookmarks_path = appdirs.user_data_dir("Firefox")

    for root, dirs, files in os.walk(bookmarks_path + "/Profiles/"):
        for name in files:
            if name == "places.sqlite":
                print("Indexing: ", root + os.sep + name)
                return root + os.sep + name
    return None


def create_connection(db_file):
    """ create a database connection to the SQLite database
        specified by db_file
    :param db_file: database file
    :return: Connection object or None
    """
    conn = None
    try:
        conn = sqlite3.connect(db_file)
        return conn
    except sqlite3.Error as e:
        print(e)

    return conn


def create_table(conn, table_sql):
    """ create a table from the table_sql statement
    :param conn: Connection object
    :param table_sql: a CREATE TABLE statement
    :return:
    """
    c = conn.cursor()
    c.execute(table_sql)
    conn.commit()


def index_site(conn, row, verbose):
    url = row[0]
    if ".local" in url:
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
        print("E", url)
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
    "--stop-when-exists",
    default=10,
    show_default=True,
    help="Stop indexing after <int> existing sites.",
)
@click.option("-v", "--verbose", is_flag=True, help="Enables verbose mode")
def do_index(verbose, stop_when_exists):
    path = get_bookmarks_path()
    exists = 0
    if path:
        temp_path = create_temporary_copy(path)

        with create_connection(temp_path) as places:
            with closing(places.cursor()) as ff_cursor:
                ff_cursor = select_bookmarks(ff_cursor)

                if not os.path.exists("./data"):
                    os.makedirs("./data")
                with create_connection("./data/fafi.sqlite") as fafi:
                    create_table(
                        fafi,
                        "CREATE VIRTUAL TABLE IF NOT EXISTS sites USING FTS5(url, text)",
                    )

                    for row in ff_cursor:
                        o = index_site(fafi, row, verbose)
                        if o == "=":
                            exists += 1
                            if stop_when_exists != -1 and exists >= stop_when_exists:
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
    if os.path.exists("./data/fafi.sqlite"):
        with create_connection("./data/fafi.sqlite") as fafi:
            cursor = fafi.execute(
                """SELECT 
                        url, 
                        snippet(sites, 1,'[', ']', '...',32) 
                    FROM 
                        sites 
                    WHERE 
                        text MATCH ? 
                    ORDER BY 
                        rank 
                    LIMIT ?
                """,
                (query, max_results),
            )
            if cursor.rowcount == 0:
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
