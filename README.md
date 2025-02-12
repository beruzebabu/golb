# Golb

Golb is a microblogging engine (written in Go) which leverages Markdown for content creation and storage. It's designed to be lightweight, efficient, and low-bandwidth, making it ideal for resource-constrained devices and fast page load times.

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
- **Minimal Dependencies**: Utilizes Go's standard library for core functionality.
