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
    c.execute( "CREATE VIRTUAL TABLE IF NOT EXISTS sites2 USING FTS5(title, url, text, date_bm_added)")
    c.execute( "INSERT OR IGNORE INTO sites2 (url, text, date_bm_added) SELECT url, text, date_bm_added FROM sites")
    c.execute( "DELETE FROM sites")
    conn.commit()
    c.execute("VACUUM")


# execute a query on sqlite cursor
def execute_query(cursor, query, args=None):
    try:
        if args:
            cursor.execute(query, args)
        else:
            cursor.execute(query)
    except Exception as error:
        print(str(error) + "\n " + query)


def search(conn, keywords, max_results):
    cursor = conn.execute(
        """SELECT 
                title,
                url, 
                snippet(sites2, 2,'[', ']', '...',64) 
            FROM 
                sites2 
            WHERE 
                title MATCH ? OR
                text MATCH ?
            ORDER BY 
                rank 
            LIMIT ?
        """,
        (keywords, keywords, max_results),
    )
    if cursor.rowcount == 0:
        return None
    return cursor


def last_row_bm_date(conn):
    cursor = conn.execute(
        """SELECT 
                date_bm_added
            FROM 
                sites2
            ORDER by date_bm_added DESC
            LIMIT 1
        """
    )
    if cursor.rowcount == 0:
        return None

    for row in cursor:
        return row[0]
