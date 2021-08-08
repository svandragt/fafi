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

        inputbox = toga.TextInput(id='inputbox')
        inputbox.style.flex = 1
        inputbox.style.padding_bottom = 10
        inputbox.on_change = self.OnInputboxChange
        self.inputbox = inputbox

        box.add(inputbox)

        container = toga.OptionContainer()

        self.resultsbox = toga.Table(['Link', 'Match'])
        container.add('Results',self.resultsbox)

        logbox = toga.MultilineTextInput(id='logbox', readonly=True)
        logbox.style.flex = 1
        logbox.style.padding_top = 50
        self.logbox = logbox
        container.add('Logs', self.logbox)

        self.container = container
        box.add(container)

        run_group = toga.Group('Run', order=40)

        cmd_index = toga.Command(
            commands.cmd_index,
            label='Index bookmarks',
            tooltip='Tells you when it has been activated',
            shortcut=toga.Key.MOD_1+'i',
            icon='icons/pretty.png',
            group=run_group,
        )
        self.commands.add(cmd_index)

        self.main_window = toga.MainWindow(title=self.formal_name)
        self.main_window.content = box
        self.main_window.show()


def main():
    global me
    me = Fafi()
    return me
