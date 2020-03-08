# faff
Favourites Firefox indexing and search tool.

WIP:

 * Extract main text content from all bookmarks into ./data/*.txt files
 * Skips .local domains

It does not return the URLs yet, although they're the first line in the text file.

```
# Setup data directory.
mkdir data

# Install project requirements.
pipenv install

# Update path to your places.sqlite location.
nano faff.py

# Index bookmarks 
pipenv run python ./faff.py

# Search for bookmarks containing a query such as 'vpn'
grep -iR 'vpn' data
```
>data/e74187ee5762840a76b327594b9167d89440124c25cb488915bbc133e9723cbe.txt:You can configure Docker Desktop networking to work on a virtual private network (VPN). Specify a network address translation (NAT) prefix and subnet mask to enable Internet connectivity.
>data/607966c6b0fdb9918464ac5e81ff2ffda55855ea41cdbd49c21d18257c96e4df.txt:URL: https://averagelinuxuser.com/linux-vpn-server/
>data/607966c6b0fdb9918464ac5e81ff2ffda55855ea41cdbd49c21d18257c96e4df.txt:How to set up a Linux VPN server (Beginner's Guide)
>data/607966c6b0fdb9918464ac5e81ff2ffda55855ea41cdbd49c21d18257c96e4df.txt:A VPN, or Virtual Private Network, creates an encrypted tunnel between your computer and a remote server. This has two major advantages. First, you mask your real location because you will have the IP address of the VPN server. Second, all the traffic between your computer and the server is encrypted. So, if you connect to a public WiFi, your data remains safe even if it intercepted by someone. Similarly, your Internet Service provider cannot read your data.
