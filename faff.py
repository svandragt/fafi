#!/usr/bin/env python3

import os
import sqlite3
import tempfile
import shutil
from contextlib import closing
import hashlib
from subprocess import call
import newspaper
import appdirs
import argparse


faff = None

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


def index_site(conn, row):
    url = row[0]
    if ".local" in url:
        return

    c = conn.cursor()
    c.execute("SELECT url FROM sites WHERE url=?", (url,))
    if c.fetchone():
        print("=", url)
        return

    article = newspaper.Article(url)
    try:
        article.download()
        article.parse()
    except newspaper.article.ArticleException:
        print("E", url)
        return
    print("✓", url)

    c.execute("INSERT INTO sites (url, text) VALUES(?,?)", (url, article.text))
    conn.commit()


def do_index(args):
    path = get_bookmarks_path()
    if path:
        temp_path = create_temporary_copy(path)

        with create_connection(temp_path) as places:
            with closing(places.cursor()) as ff_cursor:
                ff_cursor = select_bookmarks(ff_cursor)

                if not os.path.exists("./data"):
                    os.makedirs("./data")
                with create_connection("./data/faff.sqlite") as faff:
                    create_table(
                        faff,
                        "CREATE VIRTUAL TABLE IF NOT EXISTS sites USING FTS5(url, text)",
                    )

                    for row in ff_cursor:
                        index_site(faff, row)


def do_search(query):
    print("Searching for:", query)
    if os.path.exists("./data/faff.sqlite"):
        with create_connection("./data/faff.sqlite") as faff:
            cursor = faff.execute(
                "SELECT url,text FROM sites WHERE text MATCH ? ORDER BY rank", (query,)
            )
            if cursor.rowcount == 0:
                print("No results.")
                return

            i = 1
            for row in cursor:
                print(str(i) + ")", row[0])
                i += 1


if __name__ == "__main__":
    parser = argparse.ArgumentParser(usage="faff [command] [options]")
    parser.add_argument("task", help="Index bookmarks.", nargs="?")
    parser.add_argument("value", help="Search for keywords.", nargs="?")

    args = parser.parse_args()

    if args.task == "index":
        do_index(args)
    if args.task == "search":
        do_search(args.value)
