# FRSH - File (R)Sync ssH

Utility written in Golang that supports sync (or compress and copy using TAR) over SSH/RSync from/to remote origin.

Install dependencies & build:
```sh
go mod init main
go mod vendor
go build .
```

Set up config.yml file using template:
```yml
verbose: 0                                   # 0 = no verbosity, 1 = mid verbosity, 2 = high verbosity

servers:
  myserver_name_could_be_any:
    user: username
    host: yourserver.local
    private_key: ~/somedir/id_rsa
    port: 22
  other_myserver:
    user: username
    host: yourserver.local
    private_key: ~/somedir/id_rsa
    port: 22

tar_and_copy:
  # compress and copy from server
  - server: myserver_name_could_be_any       # remote server name to work with
    filename: backup_filename_prefix1        # file prefix of output archive, final name would be:  backup_filename_prefix1_1655494056.tar.gz
    log: 'Custom log (msg could be blank)'     # optional, custom msg to be shown before run this action
    source: remote:~/some/loc/               # compress this location and copy (pay attention to 'remote:*' prefix)
    dest: /Users/mylocalusername/            # to this location (could be anything, eg. '/home/user/someloc/')
    verbose: 1                               # 0 = no verbosity, 1 = mid verbosity, 2 = high verbosity
    dry_run: false                           # do not do real changes
    exclude:                                 # exclude patterns to be skipped from archiving, relative to your source
      - '*.part'
      - 'subdir/somefile.zip'

  # compress and copy to server
  - server: other_myserver                   # different server
    filename: backup_filename_prefix2
    log: ''                                  # would print default message
    source: /Users/user/Documents            # compress LOCAL location and copy
    dest: remote:~/docsBackup/               # to THIS REMOTE (pay attention to 'remote:*' prefix)
    verbose: 1
    dry_run: false
    exclude:
      - '*.part'

sync:
  # sync from server
  - server: myserver_name_could_be_any
    log: Copying Downloads dir               # optional, custom msg to be shown before run this action
    source: remote:"~/downloads/Folder name With Spaces/"              # pay attention to (1) 'remote:*' prefix and (2) last '/' if you want to only copy child files of 'downloads' dir
    dest: "/Users/user/downloads/Folder name With Spaces"             # sync them into LOCAL 'downloads' dir
    delete_extraneous_from_dest: false       # WARN! Removes files in dest loc, that are not exists in source location
    verbose: 0
    dry_run: true
    exclude:                                 # exclude patterns to be skipped from sync, relative to your source
      - '*.part'

  # sync local to server
  - server: other_myserver
    source: /Users/user/test/                # local dir
    dest: remote:~/test123/                  # copies here (pay attention to 'remote:*' prefix)
    delete_extraneous_from_dest: false       # WARN! Removes files in dest loc, that are not exists in source location
    verbose: 0
    dry_run: true
    exclude:
      - '*.part'
```

And run to execute sync:

```sh
./main
```
