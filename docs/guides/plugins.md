# Extending okfctl with plugins

okfctl extends `git`/`kubectl`-style: an unknown subcommand `okfctl foo bar`
execs an `okfctl-foo` binary found on `PATH`, passing through `bar` plus the
remaining flags and environment (with `OKFCTL` set to the core binary's path so a
plugin can call back), and propagates the plugin's exit code. Built-in
subcommands always take precedence; an unknown subcommand with no matching plugin
produces the usual error plus a did-you-mean suggestion.

The bundled `okfctl-search` plugin is the reference example—see
[Search](search.md).

## Discover plugins

```sh
okfctl plugin list                   # okfctl-<name> executables on PATH, sorted; first-on-PATH wins
okfctl plugin list --path "$PATH"    # inspect a specific PATH
```

Executable detection uses Unix permission bits (macOS/Linux); Windows isn't yet
supported.

## Install a plugin

```sh
okfctl plugin install ./okfctl-mytool           # copy into the managed plugins dir
okfctl plugin install ./okfctl-mytool --dir ~/bin
```

The default destination is `$OKFCTL_CONFIG_HOME/plugins` (or `<user config
dir>/okfctl/plugins`), the same config-home convention `config` uses; override
with `--dir`. Put that directory on your `PATH`. The source's base name must
follow `okfctl-<name>`; the copy is written with execute bits. If the destination
isn't on your `PATH`, install prints a note to stderr so the plugin isn't
silently undiscoverable.
