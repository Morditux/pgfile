# pgfile

`pgfile` is a Go library for storing files in a PostgreSQL database using `pgx`.

## Features

- **Chunking**: Files are split into 4KB (4096 bytes) chunks and stored in a `bytea` field.
- **Habilitation**: Access management based on User and Group UUIDs.
- **Metadata**: Stores name, path, creation date, and modification date.
- **Performance**: Uses `pgxpool` for efficient connection management.

## Installation

```bash
go get github.com/Morditux/pgfile
```

## Database Schema

You need to initialize your database with the following schema:

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
```

## Usage

### Initialization

```go
pool, err := pgxpool.New(context.Background(), connString)
storage := pgfile.NewStorage(pool)
```

### Uploading a file

```go
file, err := os.Open("image.png")
meta, err := storage.Upload(ctx, "image.png", "/photos/2026", groupID, userID, file)
```

### Downloading a file

```go
err := storage.Download(ctx, fileID, userID, groupID, outWriter)
```

## License

MIT
