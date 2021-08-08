import configparser
from . import data

config = None


def get_config_path():
    return data.get_data_dir() + "/config.ini"


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

    with open(get_config_path(), 'w') as configfile:  # save
        config.write(configfile)
