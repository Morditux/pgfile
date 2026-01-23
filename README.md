# pgfile

`pgfile` is a Go library for storing files in a PostgreSQL database using `pgx`.

## Features

- **Chunked Storage**: Files are split into 1MB chunks and stored in a `bytea` field for efficient handling of large files.
- **Permission Model**: Built-in access control based on owner and group UUIDs.
- **Metadata Management**: Stores file name, virtual path, creation and modification timestamps.
- **High Performance**:
  - Uses `pgxpool` for efficient connection management.
  - **Zero-copy Upload**: Streaming uploads with buffer reuse to minimize memory allocations.
  - **Zero-copy Download**: Reads directly from network buffers when using binary protocol.
- **Thread-Safe**: All operations are safe for concurrent use.

## Installation

```bash
go get github.com/Morditux/pgfile
```

## Database Schema

Initialize your database with the schema in `schema.sql`:

```sql
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE files (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name TEXT NOT NULL,
    path TEXT NOT NULL,
    group_id UUID NOT NULL,
    owner_id UUID NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE file_chunks (
    file_id UUID REFERENCES files(id) ON DELETE CASCADE,
    sequence INT NOT NULL,
    data BYTEA NOT NULL,
    PRIMARY KEY (file_id, sequence)
);

CREATE INDEX idx_files_path ON files(path);
CREATE INDEX idx_files_habilitation ON files(group_id, owner_id);
```

## API

### Storage

The main entry point is the `Storage` type:

```go
pool, err := pgxpool.New(context.Background(), connString)
storage := pgfile.NewStorage(pool)
```

### Upload

Store a file from any `io.Reader`:

```go
file, err := os.Open("document.pdf")
defer file.Close()

meta, err := storage.Upload(ctx, "document.pdf", "/docs/2026", groupID, userID, file)
// meta.ID contains the new file's UUID
```

### Download

Retrieve a file to any `io.Writer` (permission check included):

```go
var buf bytes.Buffer
err := storage.Download(ctx, fileID, seekerUserID, seekerGroupID, &buf)
```

### GetMetadata

Retrieve file metadata:

```go
meta, err := storage.GetMetadata(ctx, fileID, seekerUserID, seekerGroupID)
fmt.Printf("File: %s, Size: created at %s\n", meta.Name, meta.CreatedAt)
```

### ListByPath

List all accessible files in a virtual directory:

```go
files, err := storage.ListByPath(ctx, "/docs/2026", seekerUserID, seekerGroupID)
for _, f := range files {
    fmt.Printf("- %s\n", f.Name)
}
```

### Delete

Remove a file and all its chunks (permission check included):

```go
err := storage.Delete(ctx, fileID, seekerUserID, seekerGroupID)
```

## Documentation

See the package documentation with `go doc github.com/Morditux/pgfile` or read `doc.go`.

## License

MIT
