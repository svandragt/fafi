"""
Import a list of URLS. Acceptable input is a text file containing one or more URLs, detected by a regex.
"""
import re

from fafi import core


def index(list):
    contents = None
    try:
        with open(list) as f:
            contents = f.read()
    except FileNotFoundError:
        print("File", list, "not found. Please check the spelling and permissions, and try again.")
        return

    # Source: https://urlregex.com/
    regex = r"http[s]?://(?:[a-zA-Z]|[0-9]|[$-_@.&+]|[!*\(\),]|(?:%[0-9a-fA-F][0-9a-fA-F]))+"
    matches = re.finditer(regex, contents, re.MULTILINE)

    for _, url in enumerate(matches, start=1):
        core.index_site(url.group())
