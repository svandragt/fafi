"""
Bookmarking application
"""
import toga
from toga.style import Pack
from travertino.constants import COLUMN

from . import commands, actions

me = None

class Fafi(toga.App):
    def AddLogLine(self, text):
        self.logbox.value += text
        self.logbox.refresh()

    def OnInputboxChange(self, sender):
        self.inputbox.refresh()
        if len(self.inputbox.value) < 3:
            return
        actions.action_search(self.inputbox.value, 7)

    def startup(self):
        """
        Construct and show the Toga application.

        Usually, you would add your application to a main content box.
        We then create a main window (with a name matching the app), and
        show the main window.
        """
        box = toga.Box(style=Pack(direction=COLUMN))
        self.inputbox = toga.TextInput(id='inputbox', placeholder='Keywords')
        self.inputbox.style.flex = 1
        self.inputbox.style.padding_bottom = 10
        self.inputbox.on_change = self.OnInputboxChange
        box.add(self.inputbox)

        self.logbox = toga.MultilineTextInput(id='logbox',readonly=True )
        self.logbox.style.flex = 1
        box.add(self.logbox)

        cmd_index = toga.Command(
            commands.cmd_index,
            label='Index bookmarks',
            tooltip='Index new bookmarks',
            shortcut=toga.Key.MOD_1 + 'i',
            icon='icons/pretty.png',
            group=toga.Group.COMMANDS,
        )
        self.commands.add(cmd_index)

        cmd_focus = toga.Command(
            commands.cmd_focus,
            label='Focus search',
            tooltip='Focus the search bar',
            shortcut=toga.Key.MOD_1 + 'l',
            icon='icons/pretty.png',
            group=toga.Group.COMMANDS,
        )
        self.commands.add(cmd_focus)

        self.main_window = toga.MainWindow(title=self.formal_name)
        self.main_window.content = box
        self.main_window.show()


def main():
    global me
    me = Fafi()
    return me
