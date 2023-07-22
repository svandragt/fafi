from flask import Flask
from flask import render_template, request

app = Flask(__name__)

@app.route('/')
def home():
	query = request.args.get('query') or ''
	return render_template('search.html', query=query)

if __name__ == '__main__':
	app.run()
