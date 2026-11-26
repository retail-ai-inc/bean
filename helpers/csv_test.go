package helpers

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// collect processes input and returns all batches sorted by Index.
func collect(t *testing.T, input string, opts CSVOptions) ([]CSVBatch, error) {
	t.Helper()
	var mu sync.Mutex
	var batches []CSVBatch
	err := ProcessCSV(context.Background(), strings.NewReader(input), opts, func(_ context.Context, b CSVBatch) error {
		mu.Lock()
		defer mu.Unlock()
		batches = append(batches, b)
		return nil
	})
	sort.Slice(batches, func(i, j int) bool { return batches[i].Index < batches[j].Index })
	return batches, err
}

func allRecords(batches []CSVBatch) [][]string {
	var out [][]string
	for _, b := range batches {
		out = append(out, b.Records...)
	}
	return out
}

func TestProcessCSV_ParsesCSV(t *testing.T) {
	input := "\xEF\xBB\xBFid,name,note\n" + // UTF-8 BOM
		"1,Alice,\"hello, world\"\n" +
		"2,Bob,\"multi\nline\"\n" +
		"3,\"Carol \"\"C\"\"\",\n"

	batches, err := collect(t, input, CSVOptions{HasHeader: true, BatchSize: 2})
	require.NoError(t, err)

	require.Len(t, batches, 2)
	assert.Equal(t, []string{"id", "name", "note"}, batches[0].Header, "the BOM must be skipped")
	assert.Equal(t, batches[0].Header, batches[1].Header)
	assert.Equal(t, int64(0), batches[0].Index)
	assert.Equal(t, int64(2), batches[1].Index)
	assert.Equal(t, [][]string{
		{"1", "Alice", "hello, world"},
		{"2", "Bob", "multi\nline"},
		{"3", `Carol "C"`, ""},
	}, allRecords(batches))
}

func TestProcessCSV_Options(t *testing.T) {
	input := "# comment\na; b;c\nd; e;f\n"
	batches, err := collect(t, input, CSVOptions{Comma: ';', Comment: '#', TrimLeadingSpace: true})
	require.NoError(t, err)
	assert.Nil(t, batches[0].Header)
	assert.Equal(t, [][]string{{"a", "b", "c"}, {"d", "e", "f"}}, allRecords(batches))

	_, err = collect(t, "a,b\n", CSVOptions{Comma: '"'})
	assert.ErrorContains(t, err, "invalid CSV delimiter")
}

func TestProcessCSV_EmptyInput(t *testing.T) {
	for _, opts := range []CSVOptions{{}, {HasHeader: true}} {
		batches, err := collect(t, "", opts)
		require.NoError(t, err)
		assert.Empty(t, batches)
	}

	batches, err := collect(t, "only,header\n", CSVOptions{HasHeader: true})
	require.NoError(t, err)
	assert.Empty(t, batches, "no batch when there is only a header")
}

func TestProcessCSV_LongLine(t *testing.T) {
	long := strings.Repeat("x", 1<<20) // longer than bufio.Scanner's 64KB default limit
	batches, err := collect(t, "1,"+long+"\n2,short\n", CSVOptions{})
	require.NoError(t, err)
	records := allRecords(batches)
	require.Len(t, records, 2)
	assert.Equal(t, long, records[0][1])
}

func TestProcessCSV_AllRecordsOnce(t *testing.T) {
	const n = 10_007
	var sb strings.Builder
	sb.WriteString("id,value\n")
	for i := range n {
		fmt.Fprintf(&sb, "%d,v%d\n", i, i)
	}

	for _, opts := range []CSVOptions{
		{HasHeader: true},
		{HasHeader: true, Workers: 8, BatchSize: 7},
		{HasHeader: true, Workers: 1, BatchSize: 100},
	} {
		batches, err := collect(t, sb.String(), opts)
		require.NoError(t, err)

		records := allRecords(batches)
		require.Len(t, records, n)
		for i, r := range records {
			require.Equal(t, fmt.Sprint(i), r[0], "record %d", i)
		}
		for _, b := range batches {
			assert.Equal(t, b.Records[0][0], fmt.Sprint(b.Index), "Index is the position of the first record")
		}
	}
}

func TestProcessCSV_SingleWorkerKeepsOrder(t *testing.T) {
	var sb strings.Builder
	for i := range 1000 {
		fmt.Fprintf(&sb, "%d\n", i)
	}
	var indexes []int64
	err := ProcessCSV(context.Background(), strings.NewReader(sb.String()), CSVOptions{Workers: 1, BatchSize: 10},
		func(_ context.Context, b CSVBatch) error {
			indexes = append(indexes, b.Index)
			return nil
		})
	require.NoError(t, err)
	assert.True(t, sort.SliceIsSorted(indexes, func(i, j int) bool { return indexes[i] < indexes[j] }))
}

// endlessCSV produces CSV records forever and counts how many bytes were read.
type endlessCSV struct{ read atomic.Int64 }

func (r *endlessCSV) Read(p []byte) (int, error) {
	line := "1,2,3\n"
	n := 0
	for n+len(line) <= len(p) {
		n += copy(p[n:], line)
	}
	r.read.Add(int64(n))
	return n, nil
}

func TestProcessCSV_HandlerErrorStopsReading(t *testing.T) {
	want := errors.New("db insert failed")
	r := &endlessCSV{}

	done := make(chan error, 1)
	go func() {
		done <- ProcessCSV(context.Background(), r, CSVOptions{Workers: 4, BatchSize: 100},
			func(_ context.Context, b CSVBatch) error {
				if b.Index >= 1000 {
					return want
				}
				return nil
			})
	}()

	select {
	case err := <-done:
		assert.ErrorIs(t, err, want)
	case <-time.After(5 * time.Second):
		t.Fatal("ProcessCSV did not stop after the handler error")
	}
	assert.Less(t, r.read.Load(), int64(10<<20), "reading must stop soon after the error")
}

func TestProcessCSV_ContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var calls atomic.Int64

	err := ProcessCSV(ctx, &endlessCSV{}, CSVOptions{Workers: 2, BatchSize: 10},
		func(ctx context.Context, b CSVBatch) error {
			if calls.Add(1) == 5 {
				cancel()
			}
			return nil
		})
	assert.ErrorIs(t, err, context.Canceled)
}

func TestProcessCSV_ParseError(t *testing.T) {
	input := "a,b\nc,d\ne,f,g\n" // the third record has a different number of fields

	_, err := collect(t, input, CSVOptions{})
	var pe *csv.ParseError
	require.ErrorAs(t, err, &pe)
	assert.Equal(t, 3, pe.Line)
	assert.ErrorIs(t, err, csv.ErrFieldCount)

	batches, err := collect(t, input, CSVOptions{FieldsPerRecord: -1})
	require.NoError(t, err, "variable number of fields is allowed with FieldsPerRecord < 0")
	assert.Len(t, allRecords(batches), 3)
}

func TestProcessCSV_HandlerPanic(t *testing.T) {
	err := ProcessCSV(context.Background(), strings.NewReader("a\nb\n"), CSVOptions{},
		func(context.Context, CSVBatch) error { panic("boom") })
	require.Error(t, err)
	assert.Contains(t, err.Error(), "handler panicked: boom")
}

func TestProcessCSV_ReaderError(t *testing.T) {
	want := errors.New("connection reset")
	r := io.MultiReader(strings.NewReader("a,b\nc,d\n"), &errReader{err: want})
	err := ProcessCSV(context.Background(), r, CSVOptions{}, func(context.Context, CSVBatch) error { return nil })
	assert.ErrorIs(t, err, want)
}

type errReader struct{ err error }

func (r *errReader) Read([]byte) (int, error) { return 0, r.err }

func TestProcessCSVFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.csv")
	require.NoError(t, os.WriteFile(path, []byte("id\n1\n2\n"), 0o600))

	var count atomic.Int64
	err := ProcessCSVFile(context.Background(), path, CSVOptions{HasHeader: true},
		func(_ context.Context, b CSVBatch) error {
			count.Add(int64(len(b.Records)))
			return nil
		})
	require.NoError(t, err)
	assert.Equal(t, int64(2), count.Load())

	err = ProcessCSVFile(context.Background(), filepath.Join(t.TempDir(), "missing.csv"), CSVOptions{},
		func(context.Context, CSVBatch) error { return nil })
	assert.ErrorIs(t, err, os.ErrNotExist)
}

func TestProcessCSV_NilHandler(t *testing.T) {
	assert.Error(t, ProcessCSV(context.Background(), strings.NewReader("a"), CSVOptions{}, nil))
}

func BenchmarkProcessCSV(b *testing.B) {
	var sb strings.Builder
	sb.WriteString("ID,Name,Email,Age,City,Score,Date\n")
	for i := range 100_000 {
		fmt.Fprintf(&sb, "%d,Name%d,user%d@example.com,%d,City%d,%d.%02d,2025-01-%02d\n",
			i, i, i, 18+i%60, i%50, i%100, i%100, 1+i%28)
	}
	data := sb.String()
	b.SetBytes(int64(len(data)))
	b.ReportAllocs()

	for b.Loop() {
		var rows atomic.Int64
		err := ProcessCSV(context.Background(), strings.NewReader(data), CSVOptions{HasHeader: true},
			func(_ context.Context, batch CSVBatch) error {
				rows.Add(int64(len(batch.Records)))
				return nil
			})
		if err != nil || rows.Load() != 100_000 {
			b.Fatal(err, rows.Load())
		}
	}
}
