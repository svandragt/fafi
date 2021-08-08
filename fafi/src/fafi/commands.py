import os
from contextlib import closing

from . import utility, firefox, configuration, app, index, db, data


def cmd_index(sender):
    print('index command')
    places_dbs = firefox.get_places_dbs()
    if places_dbs:
        # TODO remove default
        config = configuration.load_config()
        try:
            places_db = config['DEFAULT']['places_db']
        except KeyError:
            places_db = None

        # Handle profile deletion
        if not os.path.exists(places_db):
            places_db = None

        if not places_db:
            choice = utility.let_user_pick(places_dbs)
            places_db = places_dbs[choice - 1]
            configuration.save_config('places_db', places_db)

        print('Places:', places_db)

        temp_path = utility.create_temporary_copy(places_db)
        with db.connect(temp_path) as places:
            with closing(places.cursor()) as ff_cursor:
                ff_cursor = firefox.select_bookmarks(ff_cursor)

                data_path = data.data_path()
                with db.connect(data_path) as fafi:
                    db.create_table(fafi)

                    o = None

                    for row in ff_cursor:
                        o = index.index_site(fafi, row)
                        if o == "=":
                            continue
                        yield

                    if not o:
                        print('\nNothing to index.')
                        app.me.AddLogLine('Nothing to index.')
