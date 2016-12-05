# ARC-TODO

arc-todo is a pretty simple app that uses your `$EDITOR` to create maniphests on [phabricator](https://www.phacility.com/)

![demo](https://dl.dropboxusercontent.com/u/5747628/arc-todo.gif)

## Setup

### Install

From your command line:

```bash
go get -u github.com/DAddYE/arc-todo
```

You just need to set a:

1. Default editor
2. Default conduit url

### EDITOR

Make sure you have `$EDITOR` set up and the default conduit url.

Add in your `~/.bashrc` or `~/.bash_profile`:

```bash
export EDITOR=vim # or [1] subl (if you use sublime) or [2] code if you use vs code.
```

- [1] Setup vscode command: https://code.visualstudio.com/docs/setup/mac
- [2] Setup sublime command: https://www.sublimetext.com/docs/2/osx_command_line.html

### Conduit URL

From the command line:

```bash
$ arc set-config default https://<cunduit url here>
```

## License

MIT
