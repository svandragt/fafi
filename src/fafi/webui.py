import os

from flask import Flask
from flask import render_template, request

# fafi
from fafi import appdata
from fafi import db

app = Flask(__name__)

with app.app_context():
    config = appdata.load_config()

@app.route('/')
def home():
    query = request.args.get('query') or ''

    results=None
    if query:
        data_path = appdata.data_path(silent=False)
        if os.path.exists(data_path):
            with db.connect(data_path) as fafi:
                cursor = db.search(fafi, query, 99)
                results = cursor.fetchall()
    return render_template('search.html', query=query, results=results)

