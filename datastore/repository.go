package datastore

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresDataStore struct {
	cp *pgxpool.Pool // Connection pool to the PostgreSQL database
}

func (r *PostgresDataStore) GetProductLineByName(name string) (*Product_Line, error) {
	c, err := r.cp.Acquire(context.Background())
	if err != nil {
		return nil, fmt.Errorf("Error acquiring connection from pool: %w", err)
	}
	defer c.Release()

	var pl Product_Line
	var row pgx.Row

	row = c.QueryRow(context.Background(),
		"SELECT product_line_id, product_line_name, product_line_url_name FROM product_lines WHERE product_line_url_name=$1;", name,
	)

	if err := row.Scan(&pl.Id, &pl.Name, &pl.UrlName); err != nil {
		return nil, fmt.Errorf("Error scanning product line row: %w", err)
	}

	return &pl, nil
}

func (r *PostgresDataStore) GetSetsByProductLineName(name string) ([]Set, error) {
	c, err := r.cp.Acquire(context.Background())
	if err != nil {
		return nil, fmt.Errorf("Error acquiring connection from pool: %w", err)
	}
	defer c.Release()

	var count int
	if err := c.QueryRow(context.Background(), "SELECT COUNT(*) FROM sets").Scan(&count); err != nil {
		return nil, fmt.Errorf("Error counting sets:%w", err)
	}

	return nil, nil
}

func (r *PostgresDataStore) UpdateSets(sets []Set) error {
	c, err := r.cp.Acquire(context.Background())
	if err != nil {
		return fmt.Errorf("Error acquiring connection from pool: %w", err)
	}
	defer c.Release()

	batch := &pgx.Batch{}
	sql := "INSERT INTO sets (set_name, set_url_name, card_count, product_line_id) VALUES ($1, $2, $3, $4)"

	for _, set := range sets {
		batch.Queue(sql, set.Name, set.UrlName, set.Count, set.ProductLineId)
	}
	br := c.SendBatch(context.Background(), batch)
	defer br.Close()

	for i := 0; i < batch.Len(); i++ {
		_, err := br.Exec()
		if err != nil {
			return fmt.Errorf("Error executing batch insert for sets: %w", err)
		}
	}
	return nil
}
