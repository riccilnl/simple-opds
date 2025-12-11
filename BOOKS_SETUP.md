# Books Directory Setup

The `books/` directory should contain your Calibre library but is excluded from git.

## Required Structure

```
books/
├── metadata.db          # Calibre database file (required)
├── Author Name/
│   └── Book Title/
│       ├── cover.jpg    # Book cover
│       ├── book.epub    # Book file
│       └── metadata.opf # Book metadata
└── ...
```

## Setup Options

### Option 1: Copy Calibre Library
```bash
cp -r /path/to/your/calibre/library/* books/
```

### Option 2: Symbolic Link
```bash
ln -s /path/to/your/calibre/library books
```

### Option 3: Docker Volume Mount
```bash
docker run -v /path/to/your/calibre:/books calibre-opds-go
```

## Configuration

Set the environment variables:
```bash
export CALIBRE_DB_PATH=books/metadata.db
export CALIBRE_BOOKS_PATH=books
```

## Note

The `books/` directory is listed in `.gitignore` to prevent committing your personal library to the repository.
