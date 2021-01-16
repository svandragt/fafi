#!/usr/bin/env python3

import click
import os

# fafi
import appdata
import app
import cli
import db


@click.group()
def cli():
    pass


@click.command("index")
@click.option("-v", "--verbose", is_flag=True, help="Enables verbose mode")
def action_index(verbose):
    bm_dbs = appdata.get_places_dbs()
    if bm_dbs:
        i = cli.let_user_pick(bm_dbs)
        app.index_with_db(bm_dbs[i - 1], verbose)


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
