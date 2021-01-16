import appdirs
import datetime
import os
import shutil
import tempfile

# fafi
import db


def get_firefox_path():
    return appdirs.user_data_dir("Firefox")


def get_places_db():

    # set the path of firefox folder with databases
    ff_path = get_firefox_path()

    # recursively walk tha path
    for root, dirs, files in os.walk(ff_path + "/Profiles/"):
        for name in files:
            if name == "places.sqlite":
                db_path = str(root + os.sep + name).strip()
                print("Indexing: ", db_path)
                return db_path
    return None


def data_path(silent=False):
    data_dir = appdirs.user_data_dir("fafi")
    if not os.path.exists(data_dir):
        os.makedirs(data_dir)
    sqlite_path = data_dir + "/data.sqlite"
    if not silent:
        print("Using: " + sqlite_path)
    return sqlite_path


# get bookmarks from firefox sqlite database file and print all
def select_bookmarks(cursor):
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
    bm_date = get_last_row_bm_date()
    if not bm_date:
        bm_date = 100000
    d = datetime.datetime.fromtimestamp(bm_date / 1000000)
    print("Indexing bookmarks added after: " + str(d))
    db.execute_query(cursor, bookmarks_query, [bm_date])
    return cursor


def create_temporary_copy(path):
    temp_dir = tempfile.gettempdir()
    temp_path = os.path.join(temp_dir, "temp_file_name")
    shutil.copy2(path, temp_path)
    return temp_path


def get_last_row_bm_date():
    sqlite_path = data_path(silent=True)
    with db.connect(sqlite_path) as fafi:
        db.create_table(fafi)

        return db.last_row_bm_date(fafi)
