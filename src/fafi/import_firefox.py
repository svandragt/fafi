"""
Import Firefox bookmarks by indexing the user profile's placesdb.
"""

import datetime
import os
from contextlib import closing

import appdirs

from fafi import appdata, db, core, input


def _get_application_data_path():
    return appdirs.user_data_dir("Firefox")


def _get_places_dbs():
    # set the path of firefox folder with databases
    ff_path = _get_application_data_path()

    # recursively walk tha path
    db_paths = []
    for root, dirs, files in os.walk(ff_path + "/Profiles/"):
        for name in files:
            if name == "places.sqlite":
                db_path = str(root + os.sep + name).strip()
                db_paths.append(db_path)

    return db_paths


def index():
    places_dbs = _get_places_dbs()
    if places_dbs:
        config = appdata.load_config()
        try:
            places_db = config['DEFAULT']['firefox_places_db']
        except KeyError:
            try:
                places_db = config['DEFAULT']['places_db']
                appdata.save_config('places_db', None)
            except KeyError:
                places_db = None

        # Handle profile deletion
        if not os.path.exists(places_db):
            places_db = None

        if not places_db:
            choice = input.let_user_pick(places_dbs)
            places_db = places_dbs[choice - 1]
            appdata.save_config('firefox_places_db', places_db)

        print('Places:', places_db)
        _index_with_places(places_db)
    else:
        print('ERROR: Places database not found')


def _index_with_places(places_db):
    temp_path = appdata.create_temporary_copy(places_db)

    with db.connect(temp_path) as places:
        with closing(places.cursor()) as ff_cursor:
            ff_cursor = _select_bookmarks(ff_cursor)

            for row in ff_cursor:
                core.index_site(url=row[0], date_bm_added=row[2])
            else:
                print('\nAll bookmarks are indexed.')

    os.remove(temp_path)


def _select_bookmarks(cursor):
    # get bookmarks from firefox sqlite database file and print all
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
    bm_date = appdata.get_last_row_bm_date()
    if not bm_date:
        bm_date = 100000
    d = datetime.datetime.fromtimestamp(bm_date / 1000000)

    print("Indexing bookmarks added after:", str(d))
    db.execute_query(cursor, bookmarks_query, [bm_date])

    return cursor
