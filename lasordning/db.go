package main

import (
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"

	"lasordning/series"
)

// dbPath is the PocketBook library database on the device.
// On the device the SD/internal storage root is /mnt/ext1 (= D:\ from Windows).
const dbPath = "/mnt/ext1/system/explorer-3/explorer-3.db"

// wantedExts are the formats we read. Series metadata really only lives in
// epub/fb2, so we keep the list tight to avoid noise (user's choice).
var wantedExts = []string{"epub", "fb2", "fb2.zip"}

// LoadBooks opens the library DB read-only and returns all books of the wanted
// formats, with any series metadata already present in the DB filled in.
func LoadBooks(path string) ([]series.Book, error) {
	// immutable=1 opens the file read-only without needing the -wal/-shm files
	// or write access — safe even while the device holds the DB.
	dsn := fmt.Sprintf("file:%s?mode=ro&immutable=1", filepath.ToSlash(path))
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	defer db.Close()

	// Build the ext filter placeholders.
	ph := make([]string, len(wantedExts))
	args := make([]any, len(wantedExts))
	for i, e := range wantedExts {
		ph[i] = "?"
		args[i] = e
	}

	q := `
SELECT b.id, COALESCE(b.title,''), COALESCE(b.author,''),
       COALESCE(b.firstauthor,''), COALESCE(f.ext,''),
       COALESCE(b.series,''), COALESCE(b.numinseries,0)
  FROM books_impl b
  JOIN files f ON f.book_id = b.id
 WHERE f.storageid = 1
   AND lower(f.ext) IN (` + strings.Join(ph, ",") + `)
 GROUP BY b.id
 ORDER BY b.firstauthor, b.sort_title, b.id`

	rows, err := db.Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("query books: %w", err)
	}
	defer rows.Close()

	var out []series.Book
	for rows.Next() {
		var (
			b        series.Book
			sName    string
			sNum     float64
		)
		if err := rows.Scan(&b.ID, &b.Title, &b.Author, &b.FirstAuthor, &b.Ext, &sName, &sNum); err != nil {
			return nil, err
		}
		if sName != "" {
			b.Series = series.Series{Name: sName, Number: sNum, Source: series.SourceMetadata}
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// backupOnce copies the DB to <db>.bak_lasordning exactly once (never
// overwrites an existing backup), before the first time we write.
func backupOnce(path string) error {
	bak := path + ".bak_lasordning"
	if _, err := os.Stat(bak); err == nil {
		return nil // already have a backup — leave it
	}
	src, err := os.Open(path)
	if err != nil {
		return err
	}
	defer src.Close()
	dst, err := os.Create(bak)
	if err != nil {
		return err
	}
	defer dst.Close()
	if _, err := io.Copy(dst, src); err != nil {
		return err
	}
	return dst.Sync()
}

// SaveSeries writes a book's series name + number back to the device DB. It
// backs the DB up once first, then updates only the two series columns of the
// one row, inside a transaction. Everything else in the library is untouched.
func SaveSeries(path string, bookID int64, name string, number float64) error {
	if err := backupOnce(path); err != nil {
		return fmt.Errorf("backup: %w", err)
	}
	dsn := fmt.Sprintf("file:%s?mode=rw", filepath.ToSlash(path))
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return fmt.Errorf("open rw: %w", err)
	}
	defer db.Close()

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	_, err = tx.Exec(
		`UPDATE books_impl SET series = ?, numinseries = ? WHERE id = ?`,
		name, int64(number), bookID,
	)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("update: %w", err)
	}
	return tx.Commit()
}
