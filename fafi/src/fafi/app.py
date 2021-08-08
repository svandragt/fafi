"""
Bookmarking application
"""
import toga
from toga.style import Pack
from toga.style.pack import COLUMN, ROW
from contextlib import closing

import datetime
import newspaper

from fafi import db

def callback(sender):
    print("Command activated")


class Fafi(toga.App):

    def startup(self):
        """
        Construct and show the Toga application.

        Usually, you would add your application to a main content box.
        We then create a main window (with a name matching the app), and
        show the main window.
        """
        main_box = toga.Box()

        cmd_index = toga.Command(
            callback,
            label='Index bookmarks',
            tooltip='Tells you when it has been activated',
            shortcut='i',
            icon='icons/pretty.png',
            group=toga.Group.EDIT,
        )
        self.commands.add(cmd_index)

        self.main_window = toga.MainWindow(title=self.formal_name)
        self.main_window.content = main_box
        self.main_window.show()


def main():
    return Fafi()


def index_site(conn, row, verbose):
    url = row[0]
    date_bm_added = row[2]
    d = datetime.datetime.fromtimestamp(date_bm_added / 1000000)
    if any(x in url for x in [".local", ".test"]):
        print("\nS", url)
        return

    c = conn.cursor()
    c.execute("SELECT url FROM sites WHERE url=?", (url,))
    if c.fetchone():
        if verbose:
            print("\n=", url)
        return "="

    article = newspaper.Article(url)
    try:
        article.download()
        article.parse()
    except newspaper.article.ArticleException:
        print("\nE", article.download_exception_msg, article.url)
        return "E"

    print("\n✓", url, "(", str(d), ")")
    c.execute(
        "INSERT INTO sites (url, text, date_bm_added) VALUES(?,?,?)",
        (url, article.text, date_bm_added),
    )
    conn.commit()
    return "+"


def index_with_db(places_db, verbose):
    temp_path = appdata.create_temporary_copy(places_db)
    with db.connect(temp_path) as places:
        with closing(places.cursor()) as ff_cursor:
            ff_cursor = appdata.select_bookmarks(ff_cursor)

            data_path = appdata.data_path()
            with db.connect(data_path) as fafi:
                db.create_table(fafi)

                o = None

                for row in ff_cursor:
                    o = index_site(fafi, row, verbose)
                    if o == "=":
                        continue

                if not o:
                    print('\nNothing to index.')
