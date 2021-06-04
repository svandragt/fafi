#!/usr/bin/env python3

import click
import os

# fafi
import appdata
import app
import input
import db


@click.group()
def cli():
    pass


@click.command("index")
@click.option("-v", "--verbose", is_flag=True, help="Enables verbose mode")
def action_index(verbose):
    places_dbs = appdata.get_places_dbs()
    if places_dbs:
        # TODO remove default
        config = appdata.load_config()
        try:
            places_db = config['DEFAULT']['places_db']
        except KeyError:
            places_db = None

        # Handle profile deletion
        if not os.path.exists(places_db):
            places_db = None

        if not places_db:
            choice = input.let_user_pick(places_dbs)
            places_db = places_dbs[choice - 1]
            appdata.save_config('places_db', places_db)

        print('Places:', places_db)

        app.index_with_db(places_db, verbose)


@click.command("search")
@click.argument("keywords")
@click.option(
    "--max-results", default=7, show_default=True, help="Return <int> results",
)
def action_search(keywords, max_results):
    print("Searching for:", keywords)
    data_path = appdata.data_path()
    if os.path.exists(data_path):
        with db.connect(data_path) as fafi:
            cursor = db.search(fafi, keywords, max_results)
            if cursor is None:
                print("No results.")
                return

            i = 1
            for row in cursor:
                url = row[0]
                snippet = row[1].replace("\n", " ").strip()
                print(str(i) + ")", url, "\n", snippet, "\n")
                i += 1


cli.add_command(action_index)
cli.add_command(action_search)

if __name__ == "__main__":
    cli()
