#!/usr/bin/env python3
from textual import events
from textual.app import App
from textual.widgets import Header, Footer, Placeholder, ScrollView

import os

import click

# fafi
from fafi import appdata
from fafi import core
from fafi import db
from fafi import import_firefox
from fafi import import_list


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
    "--max-results", default=99, show_default=True, help="Return <int> results",
)
def action_search(keywords, max_results):
    MyApp.run(title="Fafi",keywords=keywords,max_results=max_results)


main.add_command(action_index)
main.add_command(action_search)


class MyApp(App):
    def __init__(self, *args, keywords, max_results, **kwargs):
        self.keywords = keywords
        self.max_results = max_results
        super().__init__(*args, **kwargs)

    async def on_load(self, event: events.Load) -> None:
        """Bind keys with the app loads (but before entering application mode)"""
        await self.bind("q", "quit", "Quit")
        await self.bind("escape", "quit", "Quit")

    async def on_mount(self, event: events.Mount) -> None:
        """Create and dock the widgets."""

        # A scrollview to contain the markdown file
        body = ScrollView(gutter=1)

        # Header / footer / dock
        await self.view.dock(Header(clock=False, tall=False), edge="top")
        await self.view.dock(Footer(), edge="bottom")

        # Dock the body in the remaining space
        await self.view.dock(body, edge="right")

        async def do_search() -> None:
            data_path = appdata.data_path(silent=False)
            if os.path.exists(data_path):
                with db.connect(data_path) as fafi:
                    cursor = db.search(fafi, self.keywords, self.max_results)
                    if cursor is None:
                        contents = 'No results.'
                    else:
                        contents = ''
                        i = 1
                        for row in cursor:
                            title = '' if row[0] is None else str(row[0]).upper() + '\n'
                            url = row[1]
                            snippet = row[2].replace('\n\n', '\n').strip()
                            contents += f"{i}) {title}{url}\n{snippet}\n\n"
                            i += 1
                        if i == 1:
                            contents = 'No results.'
            await body.update(contents)
            await self.set_focus(body)

        await self.call_later(do_search)


# Must be under MyApp otherwise PyInstaller gets confused!
if __name__ == "__main__":
    main()
