"""
Bookmarking application
"""
import toga
from . import commands


class Fafi(toga.App):

    def startup(self):
        """
        Construct and show the Toga application.

        Usually, you would add your application to a main content box.
        We then create a main window (with a name matching the app), and
        show the main window.
        """
        main_box = toga.Box()

        run_group = toga.Group('Run', order=40)

        cmd_index = toga.Command(
            commands.cmd_index,
            label='Index bookmarks',
            tooltip='Tells you when it has been activated',
            shortcut='i',
            icon='icons/pretty.png',
            group=run_group,
        )
        self.commands.add(cmd_index)

        self.main_window = toga.MainWindow(title=self.formal_name)
        self.main_window.content = main_box
        self.main_window.show()


def main():
    return Fafi()
