package persona1m

import (
	"context"
	"crypto/sha1" //nolint:gosec // Persona IDs must match the published dataset IDs.
	"database/sql"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/Masterminds/squirrel"
	"github.com/parquet-go/parquet-go"
	_ "modernc.org/sqlite" // Registers the SQLite driver.
)

const maxSampleSize = 10_000
const maxInt64 = uint64(1<<63 - 1)

var (
	ErrUnavailable     = errors.New("persona 1m dataset unavailable")
	ErrInvalidQuery    = errors.New("invalid persona 1m query")
	personaSlugPattern = regexp.MustCompile(`[^a-z0-9]+`)
)

type Query struct {
	SampleSize       int
	Seed             int
	Sources          []string
	DimensionFilters map[string][]string
}

type Persona struct {
	ID             string
	Source         string
	SourceRowIndex int64
	Dimensions     map[string]string
}

type Result struct {
	MatchedCount int
	Personas     []Persona
}

type Repository struct {
	directory string
	db        *sql.DB
	columns   []codeColumn
	shards    []shard
	rows      uint64
	initErr   error
}

type codeColumn struct {
	ID     string   `json:"id"`
	Values []string `json:"values"`
}

type codebook struct {
	Columns []codeColumn `json:"columns"`
}

type shard struct {
	Path     string `json:"path"`
	RowStart uint64 `json:"rowStart"`
	RowCount uint64 `json:"rowCount"`
}

type indexManifest struct {
	Rows   uint64  `json:"rows"`
	Shards []shard `json:"shards"`
}

func NewRepository(directory string) *Repository {
	repository := &Repository{directory: directory}
	if err := repository.initialize(); err != nil {
		repository.initErr = fmt.Errorf("initialize persona 1m repository: %w", err)
	}

	return repository
}

//nolint:funcorder // Keep initialization next to the constructor.
func (r *Repository) initialize() error {
	codebookData, err := os.ReadFile(filepath.Join(r.directory, "persona_codes.schema.json"))
	if err != nil {
		return err
	}

	var schema codebook
	if err := json.Unmarshal(codebookData, &schema); err != nil {
		return fmt.Errorf("decode persona codebook: %w", err)
	}

	if len(schema.Columns) == 0 {
		return errors.New("persona codebook has no columns")
	}

	indexData, err := os.ReadFile(filepath.Join(r.directory, "indexes", "manifest.json"))
	if err != nil {
		return err
	}

	var manifest indexManifest
	if err := json.Unmarshal(indexData, &manifest); err != nil {
		return fmt.Errorf("decode persona index manifest: %w", err)
	}

	if manifest.Rows == 0 || len(manifest.Shards) == 0 {
		return errors.New("persona index manifest has no rows or shards")
	}

	db, err := sql.Open("sqlite", "file:"+filepath.Join(r.directory, "indexes", "postings.sqlite")+"?mode=ro")
	if err != nil {
		return fmt.Errorf("open persona postings index: %w", err)
	}

	if err := db.Ping(); err != nil {
		if closeErr := db.Close(); closeErr != nil {
			return fmt.Errorf("ping persona postings index: %w; close index: %w", err, closeErr)
		}

		return fmt.Errorf("ping persona postings index: %w", err)
	}

	db.SetMaxOpenConns(4)

	r.db = db
	r.columns = schema.Columns
	r.shards = manifest.Shards
	r.rows = manifest.Rows
	return nil
}

func (r *Repository) Query(ctx context.Context, query Query) (Result, error) {
	if r.initErr != nil {
		return Result{}, fmt.Errorf("%w: %w", ErrUnavailable, r.initErr)
	}

	if query.SampleSize < 1 || query.SampleSize > maxSampleSize {
		return Result{}, fmt.Errorf("%w: sample size must be between 1 and %d", ErrInvalidQuery, maxSampleSize)
	}

	candidates, err := r.candidates(ctx, query)
	if err != nil {
		return Result{}, err
	}

	matchedCount := len(candidates)
	if matchedCount < query.SampleSize {
		return Result{}, fmt.Errorf(
			"%w: requested %d personas but only %d matched",
			ErrInvalidQuery,
			query.SampleSize,
			matchedCount,
		)
	}

	selected := sampleRowIDs(candidates, query.SampleSize, query.Seed)
	personas, err := r.readPersonas(ctx, selected)
	if err != nil {
		return Result{}, err
	}

	return Result{MatchedCount: matchedCount, Personas: personas}, nil
}

func (r *Repository) candidates(ctx context.Context, query Query) ([]uint64, error) {
	filters := make(map[string][]string, len(query.DimensionFilters)+1)
	for field, values := range query.DimensionFilters {
		cleaned := cleanValues(values)
		if len(cleaned) > 0 {
			filters[field] = cleaned
		}
	}

	if sources := cleanValues(query.Sources); len(sources) > 0 {
		filters["source"] = append(filters["source"], sources...)
	}

	if len(filters) == 0 {
		all := make([]uint64, r.rows)
		for index := range all {
			all[index] = uint64(index)
		}

		return all, nil
	}

	postings := make([][]uint64, 0, len(filters))
	for field, values := range filters {
		ids, err := r.rowIDsFor(ctx, field, values)
		if err != nil {
			return nil, err
		}

		if len(ids) == 0 {
			return []uint64{}, nil
		}

		postings = append(postings, ids)
	}

	sort.Slice(postings, func(i, j int) bool { return len(postings[i]) < len(postings[j]) })
	result := postings[0]
	for _, posting := range postings[1:] {
		result = intersect(result, posting)
		if len(result) == 0 {
			break
		}
	}

	return result, nil
}

func (r *Repository) rowIDsFor(ctx context.Context, field string, values []string) ([]uint64, error) {
	query, args, err := squirrel.Select("row_ids").
		From("postings").
		Where(squirrel.Eq{"field_id": field, "value": values}).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build persona postings query for %s: %w", field, err)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query persona postings for %s: %w", field, err)
	}
	defer rows.Close()

	var result []uint64
	for rows.Next() {
		var packed []byte
		if err := rows.Scan(&packed); err != nil {
			return nil, fmt.Errorf("scan persona postings for %s: %w", field, err)
		}

		result = append(result, unpackRowIDs(packed)...)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate persona postings for %s: %w", field, err)
	}

	return result, nil
}

func (r *Repository) readPersonas(ctx context.Context, rowIDs []uint64) ([]Persona, error) {
	byShard := make(map[int][]uint64)
	for _, rowID := range rowIDs {
		shardIndex := r.shardFor(rowID)
		if shardIndex < 0 {
			return nil, fmt.Errorf("%w: row %d is outside the shard manifest", ErrUnavailable, rowID)
		}

		byShard[shardIndex] = append(byShard[shardIndex], rowID)
	}

	personas := make([]Persona, 0, len(rowIDs))
	for shardIndex := range r.shards {
		shardRowIDs := byShard[shardIndex]
		if len(shardRowIDs) == 0 {
			continue
		}

		if err := ctx.Err(); err != nil {
			return nil, err
		}

		rows, err := r.readShardRows(ctx, shardIndex, shardRowIDs)
		if err != nil {
			return nil, err
		}

		personas = append(personas, rows...)
	}

	return personas, nil
}

func (r *Repository) readShardRows(ctx context.Context, shardIndex int, rowIDs []uint64) ([]Persona, error) {
	shard := r.shards[shardIndex]
	file, err := os.Open(filepath.Join(r.directory, filepath.FromSlash(shard.Path)))
	if err != nil {
		return nil, fmt.Errorf("open persona parquet shard %s: %w", shard.Path, err)
	}
	defer file.Close()

	reader := parquet.NewGenericReader[any](file)
	defer reader.Close()
	result := make([]Persona, 0, len(rowIDs))
	for _, rowID := range rowIDs {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		localRow := rowID - shard.RowStart
		if localRow > maxInt64 {
			return nil, fmt.Errorf("persona parquet row %d exceeds reader range", rowID)
		}

		if err := reader.SeekToRow(int64(localRow)); err != nil {
			return nil, fmt.Errorf("seek persona parquet row %d: %w", rowID, err)
		}

		row := make([]any, 1)
		n, err := reader.Read(row)
		if err != nil && !errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("read persona parquet row %d: %w", rowID, err)
		}

		if n != 1 {
			return nil, fmt.Errorf("read persona parquet row %d: got %d rows", rowID, n)
		}

		record, ok := row[0].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("decode persona parquet row %d: got %T", rowID, row[0])
		}

		persona, err := r.decodePersona(record)
		if err != nil {
			return nil, fmt.Errorf("decode persona parquet row %d: %w", rowID, err)
		}

		result = append(result, persona)
	}

	return result, nil
}

func (r *Repository) decodePersona(record map[string]any) (Persona, error) {
	source, ok := record["source"].(string)
	if !ok || source == "" {
		return Persona{}, errors.New("source is missing")
	}

	rowIndex, ok := asInt64(record["source_row_index"])
	if !ok || rowIndex < 0 {
		return Persona{}, errors.New("source row index is invalid")
	}

	attributes, ok := record["attributes"].([]byte)
	if !ok || len(attributes) != 645 {
		return Persona{}, fmt.Errorf("attributes has length %d, want 645", len(attributes))
	}

	nullBitmap, _ := record["null_bitmap"].([]byte)
	dimensions := make(map[string]string)
	for index, column := range r.columns {
		if nullBitmap != nil && index/8 < len(nullBitmap) && nullBitmap[index/8]&(1<<uint(index%8)) != 0 {
			continue
		}

		code := attributes[index/2]
		if index%2 == 0 {
			code &= 0x0f
		} else {
			code >>= 4
		}

		if int(code) < len(column.Values) {
			dimensions[column.ID] = column.Values[code]
		}
	}

	applyOverrides(dimensions, record["attribute_overrides"], r.columns)
	return Persona{
		ID:             personaID(source, rowIndex),
		Source:         source,
		SourceRowIndex: rowIndex,
		Dimensions:     dimensions,
	}, nil
}

func (r *Repository) shardFor(rowID uint64) int {
	low, high := 0, len(r.shards)
	for low < high {
		middle := low + (high-low)/2
		shard := r.shards[middle]
		if rowID < shard.RowStart {
			high = middle
			continue
		}

		if rowID >= shard.RowStart+shard.RowCount {
			low = middle + 1
			continue
		}

		return middle
	}

	return -1
}

func sampleRowIDs(candidates []uint64, sampleSize, seed int) []uint64 {
	random := rand.New(rand.NewSource(int64(seed))) //nolint:gosec // A deterministic seed is part of the query contract.
	permutation := random.Perm(len(candidates))
	selected := make([]uint64, sampleSize)
	for index := range selected {
		selected[index] = candidates[permutation[index]]
	}

	sort.Slice(selected, func(i, j int) bool { return selected[i] < selected[j] })
	return selected
}

func intersect(left, right []uint64) []uint64 {
	result := make([]uint64, 0, minInt(len(left), len(right)))
	leftIndex, rightIndex := 0, 0
	for leftIndex < len(left) && rightIndex < len(right) {
		switch {
		case left[leftIndex] < right[rightIndex]:
			leftIndex++
		case left[leftIndex] > right[rightIndex]:
			rightIndex++
		default:
			result = append(result, left[leftIndex])
			leftIndex++
			rightIndex++
		}
	}

	return result
}

func unpackRowIDs(packed []byte) []uint64 {
	if len(packed)%4 != 0 {
		return nil
	}

	result := make([]uint64, len(packed)/4)
	for index := range result {
		result[index] = uint64(binary.LittleEndian.Uint32(packed[index*4:]))
	}

	return result
}

func applyOverrides(dimensions map[string]string, raw any, columns []codeColumn) {
	items, ok := raw.([]any)
	if !ok {
		return
	}

	for _, item := range items {
		values, ok := item.(map[string]any)
		if !ok {
			continue
		}

		fieldIndex, ok := asInt64(values["field_index"])
		value, valueOK := values["value"].(string)
		if !ok || !valueOK || fieldIndex < 0 || int(fieldIndex) >= len(columns) {
			continue
		}

		dimensions[columns[fieldIndex].ID] = value
	}
}

func personaID(source string, rowIndex int64) string {
	digest := sha1.Sum([]byte(fmt.Sprintf("%s:%d", source, rowIndex))) //nolint:gosec // Must match the published persona ID algorithm.
	slug := personaSlugPattern.ReplaceAllString(strings.ToLower(source), "-")
	slug = strings.Trim(slug, "-")
	if slug == "" {
		slug = "row"
	}

	return slug + "-" + hex.EncodeToString(digest[:])[:12]
}

func cleanValues(values []string) []string {
	cleaned := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}

		if _, exists := seen[value]; exists {
			continue
		}

		seen[value] = struct{}{}
		cleaned = append(cleaned, value)
	}

	return cleaned
}

func asInt64(value any) (int64, bool) {
	switch value := value.(type) {
	case int:
		return int64(value), true
	case int32:
		return int64(value), true
	case int64:
		return value, true
	case uint:
		if uint64(value) > maxInt64 {
			return 0, false
		}

		return int64(value), true
	case uint32:
		return int64(value), true
	case uint64:
		if value > maxInt64 {
			return 0, false
		}

		return int64(value), true
	default:
		return 0, false
	}
}

func minInt(left, right int) int {
	if left < right {
		return left
	}

	return right
}
