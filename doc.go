// Package pgfile provides a Go library for storing and retrieving files
// in a PostgreSQL database using the pgx driver.
//
// # Overview
//
// pgfile enables applications to store arbitrary files directly in PostgreSQL,
// leveraging the database's transactional guarantees, backup infrastructure,
// and access control mechanisms. Files are automatically chunked into 1MB segments
// and stored in a dedicated table with foreign key relationships to file metadata.
//
// # Key Features
//
//   - Chunked Storage: Files are split into 1MB chunks (configurable via ChunkSize constant)
//     and stored as bytea fields, enabling efficient storage and retrieval of large files.
//
//   - Permission Model: Built-in access control based on owner (user) and group UUIDs.
//     All operations verify that the requester has permission to access the file,
//     either as the owner or as a member of the file's group.
//
//   - Connection Pooling: Uses pgxpool for efficient connection management,
//     suitable for high-concurrency applications.
//
//   - Zero-Copy Upload: Uses pgx.CopyFrom with a streaming source that reuses buffers,
//     minimizing memory allocations during upload.
//
//   - Zero-Copy Download: When PostgreSQL returns data in binary format,
//     chunks are read directly from network buffers without additional copying.
//
// # Database Schema
//
// The library requires two tables to be created in your PostgreSQL database:
//
//	CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
//
//	CREATE TABLE files (
//	    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
//	    name TEXT NOT NULL,
//	    path TEXT NOT NULL,
//	    group_id UUID NOT NULL,
//	    owner_id UUID NOT NULL,
//	    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
//	    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
//	);
//
//	CREATE TABLE file_chunks (
//	    file_id UUID REFERENCES files(id) ON DELETE CASCADE,
//	    sequence INT NOT NULL,
//	    data BYTEA NOT NULL,
//	    PRIMARY KEY (file_id, sequence)
//	);
//
//	CREATE INDEX idx_files_path ON files(path);
//	CREATE INDEX idx_files_habilitation ON files(group_id, owner_id);
//
// The file_chunks table uses ON DELETE CASCADE to automatically remove chunks
// when the parent file is deleted.
//
// # Usage
//
// Initialize the storage with a connection pool:
//
//	pool, err := pgxpool.New(context.Background(), "postgres://user:pass@localhost/db")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer pool.Close()
//
//	storage := pgfile.NewStorage(pool)
//
// Upload a file from any io.Reader:
//
//	file, err := os.Open("document.pdf")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer file.Close()
//
//	meta, err := storage.Upload(ctx, "document.pdf", "/documents/2026", groupID, userID, file)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Printf("Uploaded file with ID: %s\n", meta.ID)
//
// Download a file to any io.Writer:
//
//	var buf bytes.Buffer
//	err := storage.Download(ctx, fileID, seekerUserID, seekerGroupID, &buf)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	// buf now contains the file contents
//
// Retrieve file metadata:
//
//	meta, err := storage.GetMetadata(ctx, fileID, seekerUserID, seekerGroupID)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Printf("File: %s, Created: %s\n", meta.Name, meta.CreatedAt)
//
// List files in a directory:
//
//	files, err := storage.ListByPath(ctx, "/documents/2026", seekerUserID, seekerGroupID)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	for _, f := range files {
//	    fmt.Printf("- %s (ID: %s)\n", f.Name, f.ID)
//	}
//
// Delete a file:
//
//	err := storage.Delete(ctx, fileID, seekerUserID, seekerGroupID)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
// # Types
//
// [Storage] is the main type providing all file operations.
// [FileMetadata] contains file information including ID, name, path, owner, group, and timestamps.
// [ReaderSource] implements pgx.CopyFromSource for efficient streaming uploads.
//
// # Thread Safety
//
// All Storage methods are safe for concurrent use. The underlying pgxpool handles
// connection management and synchronization.
//
// # Error Handling
//
// All methods return descriptive errors wrapped with context. Permission errors
// return "access denied" or "file not found or access denied" messages.
package pgfile
