upr
===

A command line tool to manipulate pull requests on Github.
	
This tool is designed to be integrated into a CI implementation
in order to update the Status or add a Comment.


Get It
------

Cross compiled binaries are available in the `bin` directory.  Simply download the correct binary for your system and run it.

```
$ wget -O upr https://github.com/swill/upr/blob/master/bin/upr_<os>_<arch>
```


Usage
-----
```
$ upr status -h
Add or update a pull request status on Github.

This command allows an arbitrary CI implementation to
post back the status of its run to the pull request
related to the commit the CI was run against.

Usage:
  upr status [flags]

Flags:
  -c, --commit string    commit to associate the status with
  -x, --context string   the contextual identifier for this status
  -d, --desc string      a short description of the status
  -s, --state string     pull request state (pending | success | failure | error)
  -u, --url string       a reference url for more information about this status

Global Flags:
      --config string   config file (default is $HOME/.upr.yaml or ./.upr.yaml)
  -o, --owner string    owner of the repo you are working with
  -r, --repo string     name of the repo you are working with
  -t, --token string    Github access token (https://github.com/settings/tokens)
```


Config File
-----------
By default, a config file at either `$HOME/.upr.yaml` or `./.upr.yaml` will be automatically picked up.  You can also specify your own config file by passing in the `--config` flag.

The following config file formats are supported: JSON, YAML, TOML and HCL