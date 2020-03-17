import sqlite3


def create_connection(db_file):
    """ create a database connection to the SQLite database
        specified by db_file
    :param db_file: database file
    :return: Connection object or None
    """
    conn = None
    try:
        conn = sqlite3.connect(db_file)
        return conn
    except sqlite3.Error as e:
        print(e)

    return conn


def create_table(conn, table_sql):
    """ create a table from the table_sql statement
    :param conn: Connection object
    :param table_sql: a CREATE TABLE statement
    :return:
    """
    c = conn.cursor()
    c.execute(table_sql)
    conn.commit()


# execute a query on sqlite cursor
def execute_query(cursor, query):
    try:
        cursor.execute(query)
    except Exception as error:
        print(str(error) + "\n " + query)
