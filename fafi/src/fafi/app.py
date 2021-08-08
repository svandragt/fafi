"""
Bookmarking application
"""
import toga
from toga.style import Pack
from travertino.constants import COLUMN

from . import commands

me = None

class Fafi(toga.App):
    def AddLogLine(self, *args):
        text = ''
        if type(args) is tuple:
            print(args)
            text = ' '.join(args)
        self.logbox.value += text
        self.logbox.refresh()

    def OnInputboxChange(self, sender):
        self.inputbox.refresh()
        if len(self.inputbox.value) < 3:
            return
        commands.actions.action_search(self.inputbox.value, 7)

    def OnInputboxLoseFocus(self,sender):
        self.inputbox.refresh()
        print('lost focus')

    def startup(self):
        """
        Construct and show the Toga application.

        Usually, you would add your application to a main content box.
        We then create a main window (with a name matching the app), and
        show the main window.
        """
        box = toga.Box(style=Pack(direction=COLUMN))
        self.inputbox = toga.TextInput(id='inputbox')
        self.inputbox.style.flex = 1
        self.inputbox.on_change = self.OnInputboxChange
        self.inputbox.on_lose_focus = self.OnInputboxLoseFocus
        box.add(self.inputbox)

        self.logbox = toga.MultilineTextInput(id='logbox',readonly=True )
        self.logbox.style.flex = 1
        self.logbox.style.padding_top = 50
        box.add(self.logbox)

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

        cmd_test = toga.Command(
            commands.cmd_test,
            label='Set logBox',
            tooltip='Tells you when it has been activated',
            shortcut='t',
            icon='icons/pretty.png',
            group=run_group,
        )
        self.commands.add(cmd_test)

        self.main_window = toga.MainWindow(title=self.formal_name)
        self.main_window.content = box
        self.main_window.show()


def main():
    global me
    me = Fafi()
    return me
