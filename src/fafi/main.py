#!/usr/bin/env python3

import os

import click

# fafi
from . import appdata
from . import db
from . import import_firefox
from . import core
from . import import_list


@click.group()
def main():
    pass

@click.command("index")
@click.option("--firefox", required=False, is_flag=True, help="Import Firefox profile.")
@click.option("--url", required=False, default=None, help="Import single URL.")
@click.option("--list", required=False, default=None, help="Import list of URLs.")
def action_index(url, list, firefox):
    with db.connect(appdata.data_path(silent=False)) as fafi:
        db.create_table(fafi)

    if firefox:
        import_firefox.index()
    if list:
        import_list.index(list)
    if url:
        core.index_site(url)


@click.command("search")
@click.argument("keywords")
@click.option(
    "--max-results", default=7, show_default=True, help="Return <int> results",
)
def action_search(keywords, max_results):
    print("Searching for:", keywords)
    data_path = appdata.data_path(silent=False)
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
