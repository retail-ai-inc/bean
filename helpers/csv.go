// MIT License

// Copyright (c) The RAI Authors

// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:

// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.

// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package helpers

import (
	"bufio"
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
	"runtime/debug"

	"golang.org/x/sync/errgroup"
)

const (
	defaultCSVBatchSize = 1000
	csvReadBufferSize   = 64 * 1024
)

// CSVOptions configures `ProcessCSV`. The zero value reads a comma separated file without a header,
// in batches of 1000 records, with one worker per CPU.
type CSVOptions struct {
	// Workers is the number of goroutines calling the handler concurrently. Default: runtime.NumCPU().
	// Use 1 to process the batches in file order.
	Workers int
	// BatchSize is the maximum number of records passed to one handler call. Default: 1000.
	BatchSize int
	// HasHeader treats the first record as the header: it is not passed as a record, but in `CSVBatch.Header`.
	HasHeader bool

	// The options below are passed to `encoding/csv.Reader`.

	// Comma is the field delimiter. Default: ','.
	Comma rune
	// Comment, if not 0, is the comment character: lines starting with it are ignored.
	Comment rune
	// FieldsPerRecord is the number of fields each record must have: 0 means the number of fields of the first
	// record, a negative value means variable. A record with a different number of fields is an error.
	FieldsPerRecord int
	// LazyQuotes allows a quote to appear in an unquoted field and a non-doubled quote in a quoted field.
	LazyQuotes bool
	// TrimLeadingSpace ignores leading white space in a field.
	TrimLeadingSpace bool
}

// CSVBatch is a batch of records passed to the `ProcessCSV` handler.
type CSVBatch struct {
	// Header is the header record if `CSVOptions.HasHeader` is true, otherwise nil. It is shared by all
	// batches and must not be modified.
	Header []string
	// Records are the records of the batch. The handler may keep them after it returns.
	Records [][]string
	// Index is the 0-based position of the first record of the batch in the file (the header excluded).
	Index int64
}

// ProcessCSV reads CSV records from r and calls handle with batches of records, from several goroutines
// concurrently. It is meant for large files: memory use is bounded by about (2 x Workers + 1) batches,
// whatever the file size, and the file is parsed by `encoding/csv` (quoted fields, embedded newlines,
// no line length limit). A leading UTF-8 BOM is skipped.
//
// The batches are handled concurrently and therefore not in file order, unless `Workers` is 1; use
// `CSVBatch.Index` to know the position of a batch. handle must be safe for concurrent use.
//
// Processing stops at the first error: a CSV parse error (`*csv.ParseError`, with the line number), an error
// returned by handle, a panic in handle (returned as an error with the stack), or the cancellation of ctx.
// ProcessCSV returns that error once every running handler has returned. The ctx passed to handle is
// canceled when processing stops, so long running handlers can stop early.
func ProcessCSV(ctx context.Context, r io.Reader, opts CSVOptions, handle func(ctx context.Context, batch CSVBatch) error) error {
	if handle == nil {
		return errors.New("helpers: ProcessCSV handler is nil")
	}
	workers := opts.Workers
	if workers <= 0 {
		workers = runtime.NumCPU()
	}
	batchSize := opts.BatchSize
	if batchSize <= 0 {
		batchSize = defaultCSVBatchSize
	}

	reader, err := newCSVReader(r, opts)
	if err != nil {
		return err
	}

	g, gctx := errgroup.WithContext(ctx)
	batches := make(chan CSVBatch, workers)

	// Reader: parse records and send them in batches. The channel buffer bounds the memory use.
	g.Go(func() error {
		defer close(batches)

		var header []string
		if opts.HasHeader {
			h, err := reader.Read()
			if errors.Is(err, io.EOF) {
				return nil
			}
			if err != nil {
				return err
			}
			header = h
		}

		var index int64
		for {
			// Stop reading as soon as processing stopped (handler error, panic or ctx canceled).
			if err := gctx.Err(); err != nil {
				return context.Cause(gctx)
			}

			records := make([][]string, 0, batchSize)
			for len(records) < batchSize {
				record, err := reader.Read()
				if errors.Is(err, io.EOF) {
					break
				}
				if err != nil {
					return err
				}
				records = append(records, record)
			}
			if len(records) == 0 {
				return nil
			}

			select {
			case batches <- CSVBatch{Header: header, Records: records, Index: index}:
			case <-gctx.Done():
				return context.Cause(gctx)
			}
			index += int64(len(records))

			if len(records) < batchSize {
				return nil // EOF
			}
		}
	})

	// Workers: call the handler for each batch.
	for range workers {
		g.Go(func() (err error) {
			defer func() {
				if r := recover(); r != nil {
					err = fmt.Errorf("helpers: ProcessCSV handler panicked: %v\n%s", r, debug.Stack())
				}
			}()
			for batch := range batches {
				if err := gctx.Err(); err != nil {
					return context.Cause(gctx)
				}
				if err := handle(gctx, batch); err != nil {
					return err
				}
			}
			return nil
		})
	}

	return g.Wait()
}

// ProcessCSVFile opens the file at path and processes it with `ProcessCSV`.
func ProcessCSVFile(ctx context.Context, path string, opts CSVOptions, handle func(ctx context.Context, batch CSVBatch) error) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	return ProcessCSV(ctx, f, opts, handle)
}

func newCSVReader(r io.Reader, opts CSVOptions) (*csv.Reader, error) {
	br := bufio.NewReaderSize(r, csvReadBufferSize)
	// Skip the UTF-8 BOM written by e.g. Excel, otherwise it ends up in the first field.
	if bom, err := br.Peek(3); err == nil && bytes.Equal(bom, []byte{0xEF, 0xBB, 0xBF}) {
		_, _ = br.Discard(3)
	}

	reader := csv.NewReader(br)
	if opts.Comma != 0 {
		reader.Comma = opts.Comma
	}
	reader.Comment = opts.Comment
	reader.FieldsPerRecord = opts.FieldsPerRecord
	reader.LazyQuotes = opts.LazyQuotes
	reader.TrimLeadingSpace = opts.TrimLeadingSpace
	// Records are handed to other goroutines, so each one must have its own backing array.
	reader.ReuseRecord = false

	if reader.Comma == reader.Comment || !validCSVDelimiter(reader.Comma) ||
		(reader.Comment != 0 && !validCSVDelimiter(reader.Comment)) {
		return nil, fmt.Errorf("helpers: invalid CSV delimiter %q or comment character %q", reader.Comma, reader.Comment)
	}
	return reader, nil
}

// validCSVDelimiter mirrors the rule of encoding/csv, so an invalid option is reported before reading.
func validCSVDelimiter(r rune) bool {
	return r != 0 && r != '"' && r != '\r' && r != '\n' && r != 0xFFFD && (r < 0xD800 || r > 0xDFFF)
}
