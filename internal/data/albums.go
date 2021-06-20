package data

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/lib/pq"
	"musicalbums.spafinal.net/internal/validator"
	"time"
)

type Album struct {
	ID        int64     `json:"id"`
	CreatedAt time.Time `json:"-"`
	Title     string    `json:"title"`
	Year      int32     `json:"year,omitempty"`
	Runtime   Runtime   `json:"runtime,omitempty"`
	Genres    []string  `json:"genres,omitempty"`
	Version   int32     `json:"version"`
}

func ValidateAlbum(v *validator.Validator, album *Album) {
	v.Check(album.Title != "", "title", "must be provided")
	v.Check(len(album.Title) <= 500, "title", "must not be more than 500 bytes long")
	v.Check(album.Year != 0, "year", "must be provided")
	v.Check(album.Year >= 1888, "year", "must be greater than 1888")
	v.Check(album.Year <= int32(time.Now().Year()), "year", "must not be in the future")
	v.Check(album.Runtime != 0, "runtime", "must be provided")
	v.Check(album.Runtime > 0, "runtime", "must be a positive integer")
	v.Check(album.Genres != nil, "genres", "must be provided")
	v.Check(len(album.Genres) >= 1, "genres", "must contain at least 1 genre")
	v.Check(len(album.Genres) <= 5, "genres", "must not contain more than 5 genres")
	v.Check(validator.Unique(album.Genres), "genres", "must not contain duplicate values")
}

type AlbumModel struct {
	DB *sql.DB
}

func (a AlbumModel) Insert(album *Album) error {
	query := `
		INSERT INTO albums (title, year, runtime, genres)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at, version`

	args := []interface{}{album.Title, album.Year, album.Runtime, pq.Array(album.Genres)}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	return a.DB.QueryRowContext(ctx, query, args...).Scan(&album.ID, &album.CreatedAt, &album.Version)
}

func (a AlbumModel) Get(id int64) (*Album, error) {
	if id < 1 {
		return nil, ErrRecordNotFound
	}

	query := `
		SELECT id, created_at, title, year, runtime, genres, version
		FROM albums
		WHERE id = $1`

	var album Album

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)

	defer cancel()

	err := a.DB.QueryRowContext(ctx, query, id).Scan(
		&album.ID,
		&album.CreatedAt,
		&album.Title,
		&album.Year,
		&album.Runtime,
		pq.Array(&album.Genres),
		&album.Version,
	)

	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrRecordNotFound
		default:
			return nil, err
		}
	}

	return &album, nil
}

func (a AlbumModel) Update(album *Album) error {
	query := `
		UPDATE albums
		SET title = $1, year = $2, runtime = $3, genres = $4, version = version + 1
		WHERE id = $5 AND version = $6
		RETURNING version`

	args := []interface{}{
		album.Title,
		album.Year,
		album.Runtime,
		pq.Array(album.Genres),
		album.ID,
		album.Version,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := a.DB.QueryRowContext(ctx, query, args...).Scan(&album.Version)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return ErrEditConflict
		default:
			return err
		}
	}

	return nil
}

func (a AlbumModel) Delete(id int64) error {
	if id < 1 {
		return ErrRecordNotFound
	}

	query := `
		DELETE FROM albums
		WHERE id = $1`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	result, err := a.DB.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return ErrRecordNotFound
	}
	return nil
}

func (a AlbumModel) GetAll(title string, genres []string, filters Filters) ([]*Album, Metadata, error) {
	query := fmt.Sprintf(`
		SELECT count(*) OVER(), id, created_at, title, year, runtime, genres, version
		FROM albums
		WHERE (to_tvselector('simple', title) @@ plainto_tsquery OR $1 = '')
		AND (genres @> $2 OR $2 = '{}')
		ORDER BY %s %s, id ASC
		LIMIT $3 OFFSET $4`, filters.sortColumn(), filters.sortDirection())

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	args := []interface{}{title, pq.Array(genres), filters.limit(), filters.offset()}

	rows, err := a.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, Metadata{}, err
	}

	defer rows.Close()

	totalRecords := 0
	albums := []*Album{}

	for rows.Next() {
		var album Album

		err := rows.Scan(
			&totalRecords,
			&album.ID,
			&album.CreatedAt,
			&album.Title,
			&album.Year,
			&album.Runtime,
			pq.Array(&album.Genres),
			&album.Version,
		)
		if err != nil {
			return nil, Metadata{}, err
		}

		albums = append(albums, &album)
	}

	if err = rows.Err(); err != nil {
		return nil, Metadata{}, err
	}

	metadata := calculateMetadata(totalRecords, filters.Page, filters.PageSize)

	return albums, metadata, nil
}
