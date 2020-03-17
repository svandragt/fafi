import appdirs
import os
import shutil
import tempfile

# fafi
import db


def bookmarks_path():
    return appdirs.user_data_dir("Firefox")


def get_bookmarks_db():

    # set the path of firefox folder with databases
    bm_path = bookmarks_path()

    # recursively walk tha path
    for root, dirs, files in os.walk(bm_path + "/Profiles/"):
        for name in files:
            if name == "places.sqlite":
                db_path = str(root + os.sep + name).strip()
                print("Indexing: ", db_path)
                return db_path
    return None


def db_path():
    data_dir = appdirs.user_data_dir("fafi")
    if not os.path.exists(data_dir):
        os.makedirs(data_dir)
    db_path = data_dir + "/data.sqlite"
    print("Using: " + db_path)
    return db_path


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
    db.execute_query(cursor, bookmarks_query)
    return cursor


def create_temporary_copy(path):
    temp_dir = tempfile.gettempdir()
    temp_path = os.path.join(temp_dir, "temp_file_name")
    shutil.copy2(path, temp_path)
    return temp_path
