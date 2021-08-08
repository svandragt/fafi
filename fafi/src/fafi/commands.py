import os

from . import actions, utility, firefox, configuration


def cmd_search(sender):
    print('search command')


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

        actions.action_index_with_db(places_db)
