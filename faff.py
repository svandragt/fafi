import os
import sqlite3
import tempfile
import shutil
from contextlib import closing
import hashlib
from subprocess import call
import newspaper

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
        visit_count>0
        and moz_places.url like 'http%'
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
    bookmarks_path = "/home/sander/.mozilla/firefox/"
    # get firefox profile
    profiles = [i for i in os.listdir(bookmarks_path) if i.endswith(".default-release")]
    # get sqlite database of firefox bookmarks
    sqlite_path = bookmarks_path + profiles[0] + "/places.sqlite"
    if os.path.exists(sqlite_path):
        return sqlite_path
    else:
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


def create_table(conn, create_table_sql):
    """ create a table from the create_table_sql statement
    :param conn: Connection object
    :param create_table_sql: a CREATE TABLE statement
    :return:
    """
    try:
        c = conn.cursor()
        c.execute(create_table_sql)
    except sqlite3.Error as e:
        print(e)


def index(row):
    url = row[0]
    if ".local" in url:
        return
    h = hashlib.sha256(url.encode("utf-8")).hexdigest()
    filename = "./data/" + h + ".txt"
    if os.path.exists(filename) is False:
        article = newspaper.Article(url)
        try:
            article.download()
            article.parse()
        except newspaper.article.ArticleException:
            print("E", url)
            return
        print("✓", url)

        with open(filename, "w") as f:
            f.write("URL: " + url + "\n\n")

            f.write(article.text)


if __name__ == "__main__":
    path = get_bookmarks_path()
    if path:
        print("Connecting...")
        temp_path = create_temporary_copy(path)

        with create_connection(temp_path) as places:
            with closing(places.cursor()) as ff_cursor:
                ff_cursor = select_bookmarks(ff_cursor)

                for row in ff_cursor:
                    index(row)

