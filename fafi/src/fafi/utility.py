import os
import shutil
import tempfile


def let_user_pick(options):
    print("Please choose:")
    for idx, element in enumerate(options):
        print("{}) {}".format(idx + 1, element))
    i = input("Enter number: ")
    try:
        if 0 < int(i) <= len(options):
            return int(i)
    except IndexError:
        pass
    return None


def create_temporary_copy(path):
    temp_dir = tempfile.gettempdir()
    temp_path = os.path.join(temp_dir, "temp_file_name")
    shutil.copy2(path, temp_path)
    return temp_path
