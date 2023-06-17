import configparser
import os
import shutil
import tempfile

import appdirs

# fafi
from fafi import db

config = None


def data_path(silent=True):
    sqlite_path = get_data_dir() + "/data.sqlite"
    if not silent:
        print("Store: " + sqlite_path)
    return sqlite_path


def get_data_dir():
    data_dir = appdirs.user_data_dir("fafi")
    if not os.path.exists(data_dir):
        os.makedirs(data_dir)
    return data_dir


def create_temporary_copy(path):
    temp_dir = tempfile.gettempdir()
    temp_path = os.path.join(temp_dir, "temp_file_name")
    shutil.copy2(path, temp_path)
    return temp_path


def get_last_row_bm_date():
    sqlite_path = data_path()
    with db.connect(sqlite_path) as fafi:
        return db.last_row_bm_date(fafi)


def get_config_path():
    return get_data_dir() + "/config.ini"


def load_config():
    global config

    if not config:
        config = configparser.ConfigParser()
        config.read(get_config_path())

    return config


def save_config(name, value):
    global config
    config = load_config()

    config['DEFAULT'][name] = value

    # Handle option migration
    if value is None:
        config.remove_option('DEFAULT', name)

    with open(get_config_path(), 'w') as configfile:  # save
        config.write(configfile)
