import sqlite3


def connect(db_file):
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


def create_table(conn):
    """ create a table from the table_sql statement
    :param conn: Connection object
    :return:
    """
    c = conn.cursor()
    c.execute("CREATE VIRTUAL TABLE IF NOT EXISTS sites USING FTS5(url, text)")
    conn.commit()


# execute a query on sqlite cursor
def execute_query(cursor, query):
    try:
        cursor.execute(query)
    except Exception as error:
        print(str(error) + "\n " + query)


def search(conn, query, max_results):
    cursor = conn.execute(
        """SELECT 
                url, 
                snippet(sites, 1,'[', ']', '...',32) 
            FROM 
                sites 
            WHERE 
                text MATCH ? 
            ORDER BY 
                rank 
            LIMIT ?
        """,
        (query, max_results),
    )
    if cursor.rowcount == 0:
        return None
    return cursor
