# Extending okfctl with plugins

Any program named `okfctl-<name>` on your `PATH` turns into the command
`okfctl <name>`. That's how [`git`](https://git-scm.com/docs/git) and
[`kubectl`](https://kubernetes.io/docs/tasks/extend-kubectl/kubectl-plugins/)
plugins work too. You can write one in any language. This one is a three-line
shell script.

## Write and run a plugin

```text
$ printf '#!/bin/sh\necho "okfctl-hello got: $*"\necho "called back through: $(basename "$OKFCTL")"\n' > okfctl-hello
$ chmod +x okfctl-hello
$ okfctl plugin install ./okfctl-hello --dir bin
installed okfctl-hello -> bin/okfctl-hello
note: bin is not on your PATH; add it so `okfctl hello` and `okfctl plugin list` find this plugin
$ PATH="$PWD/bin:$PATH" okfctl hello world --loud
okfctl-hello got: world --loud
called back through: okfctl
```

`plugin install` copied the script into `bin` and made the copy executable. Then
it warned you that `bin` isn't on `PATH`. okfctl can't run a plugin it can't
find.

With `bin` on `PATH`, `okfctl hello` found `okfctl-hello` and handed it
everything after `hello`, flags included. The plugin also gets okfctl's
environment plus one extra variable: `OKFCTL` holds the path of the okfctl
binary that called it. Call that path and you get the same okfctl you ran.
Whatever status the plugin exits with, okfctl exits with too.

## Find the plugins on your PATH

<!-- transcript: exit 1 PATH="$PWD/bin:$PATH" okfctl helo -->
```text
$ okfctl plugin list --path "$PWD/bin"
okfctl-hello	<TMPDIR>/bin/okfctl-hello
$ PATH="$PWD/bin:$PATH" okfctl helo
okfctl: unknown command "helo" for "okfctl"

Did you mean this?
	hello

```

Leave off `--path` and `plugin list` reads your own `PATH`, sorted by name. If
two directories hold the same name, the one first on `PATH` is the one listed
and the one `okfctl <name>` will call. okfctl only counts a file with an execute
bit set (macOS and Linux have those, Windows doesn't).

The typo `helo` exited 1. okfctl drew its suggestion from the plugins on `PATH`
as well as from its own commands.

## Rules

A command okfctl ships with always beats a plugin of the same name. That's why
you type out `okfctl-search` to run the bundled search plugin: `okfctl search`
is the core word search. The [search guide](search.md) explains the two.

`plugin install` takes only a file named `okfctl-<name>`:

<!-- transcript: exit 1 okfctl plugin install ./hello-tool --dir bin -->
```text
$ cp okfctl-hello hello-tool
$ okfctl plugin install ./hello-tool --dir bin
okfctl: plugin source "hello-tool" must be named okfctl-<name>
```

Leave off `--dir` and `plugin install` copies into the `plugins` directory under
okfctl's config home. With `$OKFCTL_CONFIG_HOME` set, that's
`$OKFCTL_CONFIG_HOME/plugins`. Otherwise it's `<user config dir>/okfctl/plugins`.
Add that directory to your `PATH` one time and any plugin you install there
shows up.
