# opengrok-clj

cmdline tool based on opengrok RESTful API.

## Features:

* sort the results based on current opened file.
* cache the result in last 30 days and drop the cache when files in results changed.
* unescape html tags

## Download



## usage

```
Usage:
  opengrok-clj search [flags]

Flags:
  -d, --define string      to search define
  -h, --help               help for search
  -p, --project string     project to search (default "aosp")
  -r, --reference string   to search reference
  -f, --text string        to search text
  -w, --workfile string    current work file
  -R, --config string      config file path
```

example:

```
opengrok-clj search -R /var/opengrok/etc/configuration.xml -p aosp -f SurfaceComposerClient -w ~/workspace/aosp/frameworks/native/libs/gui/SurfaceComposerClient.cpp
```


* 
