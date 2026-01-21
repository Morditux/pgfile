package pgfile

import (
	"bytes"
	"testing"

	"github.com/google/uuid"
)

// Note: Real tests would require a database or a complex mock of pgx pool.
// Here we aim to check if the code compiles and if we can at least test some isolated logic.
// In a real scenario, we'd use pgx pool and a real postgres or pgmock.

func TestChunkSizeConstant(t *testing.T) {
	if ChunkSize != 1024*1024 {
		t.Errorf("expected ChunkSize to be 1MB, got %d", ChunkSize)
	}
}

func TestStorageStructure(t *testing.T) {
	s := NewStorage(nil)
	if s == nil {
		t.Fatal("expected NewStorage to return a non-nil Storage")
	}
}

// Basic smoke test to ensure things compile.
func TestCompiles(t *testing.T) {
	_ = &FileMetadata{
		ID:      uuid.New(),
		Name:    "test.txt",
		Path:    "/tmp",
		GroupID: uuid.New(),
		UserID:  uuid.New(),
	}
}

func TestReaderSource(t *testing.T) {
	// Create a small buffer for testing
	chunkSize := 10
	totalSize := chunkSize*3 + 5 // 3 full chunks + 1 partial
	data := make([]byte, totalSize)
	for i := range data {
		data[i] = byte(i)
	}

	// Create a ReaderSource with a mock reader
	r := bytes.NewReader(data)

	rs := &ReaderSource{
		Reader:   r,
		Buffer:   make([]byte, chunkSize),
		FileID:   uuid.New(),
		Sequence: 0,
	}

	expectedChunks := 4 // 3 full + 1 partial
	count := 0

	for rs.Next() {
		vals, err := rs.Values()
		if err != nil {
			t.Fatalf("Values() returned error at chunk %d: %v", count, err)
		}

		// Check FileID (index 0)
		if vals[0] != rs.FileID {
			t.Errorf("Chunk %d: expected FileID %v, got %v", count, rs.FileID, vals[0])
		}

		// Check Sequence (index 1) - Note: Sequence is already incremented in Values()
		// So if we start at 0, the first call returns sequence 0, and increments rs.Sequence to 1.
		// Wait, Values() returns {..., rs.Sequence, ...} then increments.
		// So returned value should match the count.
		if vals[1].(int) != count {
			t.Errorf("Chunk %d: expected sequence %d, got %d", count, count, vals[1])
		}

		// Check Data (index 2)
		chunkData := vals[2].([]byte)
		start := count * chunkSize
		end := start + len(chunkData)
		expectedData := data[start:end]

		if !bytes.Equal(chunkData, expectedData) {
			t.Errorf("Chunk %d: data mismatch", count)
		}

		count++
	}

	if rs.Err() != nil {
		t.Errorf("ReaderSource error: %v", rs.Err())
	}

	if count != expectedChunks {
		t.Errorf("Expected %d chunks, got %d", expectedChunks, count)
	}
}

// In a real implementation, I would use 'pxtest' or similar for integration tests.
// For now, I'll ensure the library is well-formed.
