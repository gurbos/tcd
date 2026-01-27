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

func (r *PostgresDataStore) GetProductLineByName(ctx context.Context, name string) (Product_Line, error) {
	var productLine Product_Line // Holds query result

	c, err := r.cp.Acquire(ctx)
	if err != nil {
		return productLine, fmt.Errorf("Error acquiring connection from pool: %w", err)
	}
	defer c.Release()

	row := c.QueryRow(ctx,
		"SELECT product_line_id, product_line_name, product_line_url_name FROM product_lines WHERE product_line_url_name=$1;", name,
	)

	if err := row.Scan(&productLine.Id, &productLine.Name, &productLine.UrlName); err != nil {
		return productLine, fmt.Errorf("Error scanning product line row: %w", err)
	}

	return productLine, nil
}

func (r *PostgresDataStore) GetSetsByProductLineName(ctx context.Context, name string) ([]Set, error) {
	c, err := r.cp.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("Error acquiring connection from pool: %w", err)
	}
	defer c.Release()

	var count int
	if err := c.QueryRow(ctx, "SELECT COUNT(*) FROM sets").Scan(&count); err != nil {
		return nil, fmt.Errorf("Error counting sets:%w", err)
	}

	return nil, nil
}

func (r *PostgresDataStore) AddProductLine(ctx context.Context, pl *Product_Line) (*Product_Line, error) {
	c, err := r.cp.Acquire(ctx)
	if err != nil {
		return pl, fmt.Errorf("error acquiring connection from pool: %w", err)
	}
	defer c.Release()

	sql := "INSERT INTO product_lines (product_line_name, product_line_url_name) " +
		"VALUES ($1, $2) RETURNING *;"
	err = c.QueryRow(ctx, sql, pl.Name, pl.UrlName).Scan(&pl.Id, &pl.Name, &pl.UrlName)
	if err != nil {
		return pl, fmt.Errorf("Error inserting product line: %w", err)
	}
	return pl, nil
}

func (r *PostgresDataStore) AddSets(ctx context.Context, sets []Set) ([]Set, error) {

	tx, err := r.cp.Begin(ctx)
	if err != nil {
		return sets, fmt.Errorf("Error beginning DB transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// String stores SQL statement  to be executed
	sql := "INSERT INTO sets (set_name, set_url_name, card_count, release_date, product_line_id) " +
		"VALUES ($1, $2, $3, $4, $5) RETURNING *;"
	batch := &pgx.Batch{} // Create a new batch for batch execution
	for _, set := range sets {
		batch.Queue(sql, set.Name, set.UrlName, set.Count, "", set.ProductLineId)
	}

	// Send the batch to the database
	batchResults := tx.SendBatch(ctx, batch)

	// Process batch results
	var isError bool // Flag to track if any errors occurred during batch execution
	for i := 0; i < batch.Len(); i++ {
		row := batchResults.QueryRow()
		err := row.Scan(
			&sets[i].Id, &sets[i].Name, &sets[i].UrlName,
			&sets[i].Count, &sets[i].ReleaseDate, &sets[i].ProductLineId,
		)
		if err != nil {
			isError = true
			fmt.Println(
				fmt.Errorf("Error scanning inserted set '%s': %w\n", sets[i].UrlName, err),
			)
		}
	}

	if isError {
		return sets, fmt.Errorf("One or more errors occurred during batch insert of sets")
	}

	if err := batchResults.Close(); err != nil {
		return sets, fmt.Errorf("Error closing batch results:")
	}

	if err := tx.Commit(ctx); err != nil {
		return sets, fmt.Errorf("Error committing DB transaction: %w", err)
	}

	return sets, nil
}

func (r *PostgresDataStore) AddProducts(ctx context.Context, products []Product) error {
	tx, err := r.cp.Begin(ctx)
	if err != nil {
		return fmt.Errorf("error beginning DB transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// SQL statement  to be executed
	sql := "INSERT INTO products (product_name, product_url_name, product_line_name, " +
		"product_line_url_name, rarity_name, custom_attributes, set_name, set_url_name, " +
		"product_number, print_edition, release_date, product_line_id, set_id) " +
		"VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13);"

	batch := &pgx.Batch{} // Create a new batch for batch execution
	for _, product := range products {
		batch.Queue(
			sql,
			product.ProductName, product.ProductUrlName, product.ProductLineName,
			product.ProductLineUrlName, product.RarityName, product.CustomAttributes,
			product.SetName, product.SetUrlName, product.ProductNumber, product.PrintEdition,
			product.ReleaseDate, product.ProductLineId, product.SetId,
		)
	}

	// Send the batch to the database
	br := tx.SendBatch(ctx, batch)
	defer br.Close()

	// Process batch results
	for i := 0; i < batch.Len(); i++ {
		_, brErr := br.Exec()
		if brErr != nil {
			return brErr
		}
	}

	if err := br.Close(); err != nil {
		return fmt.Errorf("Error closing batch results: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("Error committing DB transaction: %w", err)
	}

	return nil
}
