#!/usr/bin/env python3

import os

import click

# fafi
from . import appdata
from . import db
from . import firefox as app_firefox


@click.group()
def main():
    pass


@click.command("index")
@click.option("--firefox", required=False, is_flag=True)
def action_index(firefox=None):
    with db.connect(appdata.data_path()) as fafi:
        db.create_table(fafi)

    app_firefox.index()


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


main.add_command(action_index)
main.add_command(action_search)

if __name__ == "__main__":
    main()
