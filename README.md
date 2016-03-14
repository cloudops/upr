`upr`
=====

**Current Version: 0.2.0**

A command line tool to manipulate pull requests on Github.
	
This tool is designed to be integrated into a CI implementation
in order to update the Status of a pull request commit.


Download
--------

Cross compiled binaries are available under [Releases](https://github.com/swill/upr/releases).  Simply download the correct binary for your system and run it.


`$ upr status`
-------------

![Pull Request Status](https://objects-east.cloud.ca/v1/5ef827605f884961b94881e928e7a250/swill/pr_testing/combo_ci.png)

The `token` needs to have `repo:status` permission on the target `repo` in order for this command to work.

**Usage**
```
$ upr status -h
Add or update  the status of a pull request on Github.

This command allows an arbitrary CI implementation to
post back the status of its run to the pull request
related to the commit the CI was run against.

Usage:
  upr status [flags]

Flags:
  -x, --context string   required: the contextual identifier for this status
  -d, --desc string      a short description of the environment context
  -s, --state string     required: pull request state (pending | success | failure | error)
  -u, --url string       a reference url for more information about this status

Global Flags:
  -c, --commit string     commit you are working with
      --config string     config file (default is ./config.yaml)
      --custom_template   override the built in templates using a file at 'static/templates.tpl'
      --owner string      required: owner of the repo you are working with
      --repo string       required: name of the repo you are working with
      --token string      required: Github access token (https://github.com/settings/tokens)
```

**Example**

`./config.yaml`
``` yaml
token: xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
owner: swill
repo: upr
```

```
$ upr status -c afa097edb9b06d92cc1458f62e5ec77c808ac85f -x "CloudOps CI" -d "CI run on Xen with an Advanced Network" -s "success"
Using config file: /path/to/upr/config.yaml
2016/03/13 21:19:27 Successfully updated the status!

```

`$ upr comment`
---------------

![Pull Request Comment](https://objects-east.cloud.ca/v1/5ef827605f884961b94881e928e7a250/swill/pr_testing/comment.png)

The `token` can be your own personal token, but it needs to have at least the `public_repo` permission in order for this command to work.

**Usage**
```
$ upr comment -h
Add a comment to a pull request on Github.

This command allows an arbitrary CI implementation to
post a comment to a pull request issue thread.

Optionally, files can be made public by uploading them to
an object store using either the Swift or S3 API.

Usage:
  upr comment [flags]

Flags:
  -f, --comment_file string       required: file which includes the comment text
  -n, --pr_num int                required unless 'commit' isset: pull request number on which to comment on
  -t, --title string              the title of the comment
  -u, --uploads string            comma separated list of files or directories to be recusively uploaded
      --uploads_api string        required if 'uploads' isset: api to use to upload to an object store (s3 | swift)
  -b, --uploads_bucket string     required if 'uploads' isset: bucket to upload the files to (will be made public)
      --uploads_concurrency int   number of files to be uploaded concurrently (default 4)
      --uploads_endpoint string   required if 'uploads' isset: object store url endpoint
  -e, --uploads_expire int        optional number of days to keep the uploaded files before they are removed
      --uploads_identity string   keystone identity as 'tenant:username' if 'swift' | for 's3', use the '~/.aws/credentials' file or a 'AWS_ACCESS_KEY_ID' env var
      --uploads_region string     upload region when using the 's3' api
      --uploads_secret string     keystone password if 'swift' | for 's3', use the '~/.aws/credentials' file or a 'AWS_SECRET_ACCESS_KEY' env var

Global Flags:
  -c, --commit string     commit you are working with
      --config string     config file (default is ./config.yaml)
      --custom_template   override the built in templates using a file at 'static/templates.tpl'
      --owner string      required: owner of the repo you are working with
      --repo string       required: name of the repo you are working with
      --token string      required: Github access token (https://github.com/settings/tokens)
```

**Example**

`./config.yaml`
``` yaml
token: xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
owner: swill
repo: upr

# credentials in '~/.aws/credentials'
uploads_api: s3
uploads_endpoint: https://s3-us-west-1.amazonaws.com
uploads_region: us-west-1

#uploads_api: swift
#uploads_endpoint: https://auth-east.cloud.ca/v2.0
#uploads_identity: tenant:username
#uploads_secret: XXXXXXXXXXXXXXXX
```

```
$ upr comment -c afa097edb9b06d92cc1458f62e5ec77c808ac85f -f comment_text.md -t "Optional 'title'" -u data -b upr-example -e 7
Using config file: /path/to/upr/config.yaml
2016/03/13 23:23:13 Using bucket: upr-example
2016/03/13 23:23:13 Starting upload...  This can take a while, go get a coffee.  :)
2016/03/13 23:23:13   started: upload-expires/data/readme.md
2016/03/13 23:23:13   started: upload-expires/data/xen_advanced/env_setup.log
2016/03/13 23:23:13   started: upload-expires/data/xen_advanced/full_run.log
2016/03/13 23:23:13  uploaded: upload-expires/data/readme.md
2016/03/13 23:23:13  uploaded: upload-expires/data/xen_advanced/env_setup.log
2016/03/13 23:23:13  uploaded: upload-expires/data/xen_advanced/full_run.log
2016/03/13 23:23:13 Updating PR '2' with details.
2016/03/13 23:23:13 Finished commenting on pull request(s)!
```


Configuration
-------------
By default, a config file at `./config.yaml` will automatically be picked up if it exists.  You can also specify your own config file by passing in the `--config` flag.

The following config file formats are supported: `JSON`, `YAML`, `TOML` and `HCL`

It is recommended that you configure all of the global configuration flags, such as `token`, `owner` and `repo`, into a config file and only pass the contextual configuration flags via the command line.


Change Log
----------

### 0.2.0 - 2016/03/13
- Added the ability to `comment` on a pull request (by PR number or commit).
- Implemented both S3 and Swift object storage backends for uploading files.
- Allow for automatic expiration of uploaded files to clean up the object store after a period of time.
- Added the ability to override the default comment template with a local file.

### 0.1.0 - 2016/03/07
- Initial release of the tool.  It currently only supports updating the `status` of a pull request based on the PR commit.

