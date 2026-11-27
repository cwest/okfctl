# Remote sources: `registry` and `connect`

okfctl treats a bundle that lives somewhere else the way git treats a remote.
Name a git URL with `okfctl registry`. After that, `okfctl connect` with the
name clones the bundle if you don't have it and pulls it if you do. What
follows is that loop, start to finish, against a bundle with one node.

## Connect to a bundle

The remote in this example is a bare git repository at `./wine.git`. HTTPS and
SSH URLs work too, as does anything else `git clone` accepts.

<!-- transcript: setup
$ okfctl bundle init upstream
$ okfctl node new wine/tannin.md --type Reference --title "Tannin" --bundle upstream
$ git -C upstream init -q
$ git -C upstream add -A
$ git -C upstream -c user.name=Example -c user.email=you@example.com commit -q -m "add tannin"
$ git clone -q --bare upstream wine.git
-->
```text
$ okfctl registry add wine ./wine.git
Added remote "wine" -> ./wine.git
$ okfctl registry list
wine	./wine.git
$ okfctl connect wine
Connected ./wine.git -> wine
$ okfctl bundle info wine
nodes: 1
reserved: 3
okf_version: 0.3 (declared in index.md)
```

`registry add` stored the name and the URL. `connect wine` looked the name up
and ran `git clone`. The new directory takes its name from the source, as with
`git clone` (a trailing `.git` is dropped). What you get is an ordinary
checkout that okfctl's other commands work on like any bundle.

Someone pushes a second node to the remote. Run `connect` again:

<!-- transcript: setup
$ okfctl node new wine/oak.md --type Reference --title "Oak" --bundle upstream
$ git -C upstream add -A
$ git -C upstream -c user.name=Example -c user.email=you@example.com commit -q -m "add oak"
$ git -C upstream push -q ../wine.git HEAD
-->
```text
$ okfctl connect wine
Updated wine from ./wine.git
$ okfctl bundle info wine
nodes: 2
reserved: 3
okf_version: 0.3 (declared in index.md)
```

This time `wine` was a checkout of that source. `connect` ran
`git pull --ff-only` and got the new node. With `--ff-only` the pull never
rewrites your history: if your copy has diverged from the remote, the pull
fails and nothing in your copy changes.

## What `connect` won't touch

Point `connect` at a directory with something else in it and it refuses:

<!-- transcript: exit 1 okfctl connect wine upstream -->
```text
$ okfctl connect wine upstream
okfctl: destination upstream exists, is not empty, and is not a checkout of this source; refusing to overwrite
```

`upstream` has files in it and isn't a clone of `./wine.git`. `connect` exits 1
and doesn't change anything there. If the directory is empty or isn't there,
you get a fresh clone.

## Managing names

```text
$ okfctl registry show wine
./wine.git
$ okfctl registry remove wine
Removed remote "wine"
$ okfctl registry list
no remote sources registered
```

Run `registry add` with a name that's taken and it points that name at the new
URL. `registry rm` is short for `registry remove`. The names sit in okfctl's
config file as `registry.<name>` keys, so `okfctl config list` shows them.

You don't need the registry at all. Give `connect` a URL, plus a directory if
you want to choose one:

```sh
okfctl connect https://github.com/acme/kb.git ./kb
```

## Private repositories

okfctl does no authentication. `connect` calls your own `git`. Any URL that
`git clone` gets to with your SSH agent or credential helper works here as well.
There's no okfctl server to sign in to.
