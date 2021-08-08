import datetime
import os

import appdirs

from . import db


def get_firefox_path():
    return appdirs.user_data_dir("Firefox")


def get_places_dbs():
    # set the path of firefox folder with databases
    ff_path = get_firefox_path()

    # recursively walk tha path
    db_paths = []
    for root, dirs, files in os.walk(ff_path + "/Profiles/"):
        for name in files:
            if name == "places.sqlite":
                db_path = str(root + os.sep + name).strip()
                db_paths.append(db_path)

    return db_paths


def select_bookmarks(cursor):
    """
    get bookmarks from firefox sqlite database file and print all
    """
    bookmarks_query = """
    SELECT DISTINCT
        url, moz_places.title, dateAdded from moz_places  
    JOIN 
        moz_bookmarks on moz_bookmarks.fk=moz_places.id 
    WHERE 
        moz_places.url like 'http%' and dateAdded > ?
    ORDER BY 
        dateAdded ASC
    """
    bm_date = db.get_last_row_bm_date()
    if not bm_date:
        bm_date = 100000
    d = datetime.datetime.fromtimestamp(bm_date / 1000000)
    print("Indexing bookmarks added after: " + str(d))
    db.execute_query(cursor, bookmarks_query, [bm_date])
    return cursor
