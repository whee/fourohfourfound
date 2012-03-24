Fourohfourfound
===============

Fourohfourfound is an HTTP server intended to be used as a fallback server for
404 errors in the upstream server.

Currently, it allows you to specify a list of redirections.

nginx
-----

Some HTTP servers, such as nginx, require a configuration file change to
update the rewrite list. If updating configuration frequently isn't desirable
or possible, you can configure nginx to proxy requests to fourohfourfound if it
would otherwise return a 404.

    location / {
      ...
      error_page 404 = @fourohfourfound;
    }
    location @fourohfourfound {
      proxy_set_header X-Real-IP $remote_addr;
      proxy_pass http://localhost:4404;
    }

This assumes fourohfourfound is running on localhost:4404.

Usage
-----

Create a configuration file in JSON format:

    {
      "/source": "/destination",
      "/another-source": "/another-destination"
    }

Run `fourohfourfound`:

    $ fourohfourfound

Optional arguments are `-code=[3xx]`, `-config=[config.json]`, and `-port=[4404]`.

Redirections can be modified at runtime with PUT/DELETE:

    $ curl -X PUT -d "/somewhere-else" http://localhost:4404/new-redir
    $ curl http://localhost:4404/new-redir
    <a href="/somewhere-else">Found</a>.

    $ curl -X DELETE http://localhost:4404/new-redir
    $ curl http://localhost:4404/new-redir
    404 page not found

These are not yet persistent.

Notes
-----

Redirections are basic and not as powerful as nginx's rewrite rules. This will
likely change.

Future expansion
----------------

* Regular expressions
* Statistics/analytics
