import appdirs
import os


def data_path(silent=False):
    sqlite_path = get_data_dir() + "/data.sqlite"
    if not silent:
        print("Store: " + sqlite_path)
    return sqlite_path


def get_data_dir():
    data_dir = appdirs.user_data_dir("fafi")
    if not os.path.exists(data_dir):
        os.makedirs(data_dir)
    return data_dir
