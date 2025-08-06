## mango

<a href="https://circleci.com/gh/ddollar/mango">
  <img align="right" src="https://circleci.com/gh/ddollar/mango.svg?style=svg">
</a>

[Foreman](https://github.com/ddollar/foreman) in Go.

### Installation

[Downloads](https://dl.equinox.io/ddollar/mango/stable)

##### Compile from Source

    $ go get -u github.com/ddollar/mango

### Usage

    $ cat Procfile
    web: bin/web start -p $PORT
    worker: bin/worker queue=FOO

    $ mango start
    web    | listening on port 5000
    worker | listening to queue FOO

Use `mango help` to get a list of available commands, and `mango help
<command>` for more detailed help on a specific command.

### License

Apache 2.0 &copy; 2015 David Dollar
