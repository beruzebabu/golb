# Golb

Golb is a microblogging engine (written in Go) which leverages Markdown for content creation and storage. It's designed to be lightweight, efficient, and low-bandwidth, making it ideal for resource-constrained devices and fast page load times.

#### Golb is not (yet) ready for production as I'm still working on the basics

Golb can be run without arguments at all (view only mode), by using CLI arguments or by using environment variables.
CLI arguments have precedence over environment variables.

```
Environment variables:
- GOLB_TITLE
- GOLB_PASSWORD
- GOLB_PORT
- GOLB_POSTDIR
- GOLB_TEMPLATEDIR
- GOLB_FILEDIR
```

```
golb arguments:
  -filedir string
        specifies the directory to use for files (env: GOLB_FILEDIR) (default "files")
  -password string
        specifies the management password (env: GOLB_PASSWORD)
  -port int
        specifies the port to use, default is 8080 (env: GOLB_PORT) (default 8080)
  -postdir string
        specifies the directory to use for posts (env: GOLB_POSTDIR) (default "posts")
  -templatedir string
        specifies the directory to use for templates (env: GOLB_TEMPLATEDIR) (default "templates")
  -title string
        specifies the blog title (env: GOLB_TITLE) (default "Golb")
  -h
  		the above help text
```

*Tip: mount (blob)storage as a drive or folder and use this to store your posts (on my blog I have mounted blobstorage as the folder /posts on the pod running golb). This way, you automatically have all your posts backed up and you won't lose them when redeploying.*

**When not running in view only mode, the ```/login``` and ```/create``` endpoints are made available to manage the blog.**

## Key Features

### Markdown-Based Editing

The engine uses Markdown for post creation and editing, providing several advantages:

- **Simplicity**: Markdown's intuitive syntax allows for quick and easy content creation.
- **Portability**: Markdown files can be edited using any text editor, enabling offline work.
- **Version Control**: Markdown's plain text format is ideal for tracking changes with systems like Git.

### File-Based Storage

Posts are stored as individual Markdown files, offering benefits such as:

- **Easy Access**: Direct file system access for quick retrieval and editing.
- **Transferability**: Simple to move or backup content without database exports.
- **External Editing**: Posts can be modified using any Markdown-compatible editor.

### Efficiency and Performance

The engine is optimized for low resource usage:

- **Low Bandwidth**: Posts are rendered in pure HTML and (minified) CSS, the base does not use any JS, custom fonts or other dependencies.
- **Fast Rendering**: Go's performance combined with the output being just static HTML and CSS results in (very) quick page load times.
- **Extensibility**: The page and post rendering uses Go's template system, which means the HTML pages and CSS are fully customizable. Custom CSS, JS modules and new HTML sections can be added at will.
- **Minimal Dependencies**: Utilizes Go's standard library for core functionality. Has just 1 dependency, goldmark (for markdown processing).
