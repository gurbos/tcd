package datastore

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	UniqueViolationError = "23505"
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
		"SELECT product_line_id, product_line_name, product_line_url_name FROM product_lines WHERE product_line_name=$1;", name,
	)

	if err := row.Scan(&productLine.Id, &productLine.Name, &productLine.UrlName); err != nil {
		return productLine, fmt.Errorf("Error scanning product line row: %w", err)
	}

	return productLine, nil
}

func (r *PostgresDataStore) GetSetsByProductLineName(ctx context.Context, name string) ([]Set, error) {
	// Begin a transaction with serializable isolation level
	// which guarantees a fully consistent view of database state
	// throughout the transaction, preventing concurrency anomolies.
	txOptions := pgx.TxOptions{
		IsoLevel: pgx.Serializable,
	}
	tx, err := r.cp.BeginTx(ctx, txOptions)
	if err != nil {
		return nil, fmt.Errorf("Error acquiring connection from pool: %w", err)
	}
	defer tx.Rollback(ctx)

	// Get count of sets for the specified product line
	var setCount int
	if err := tx.QueryRow(ctx, "SELECT COUNT(*) FROM sets").Scan(&setCount); err != nil {
		return nil, fmt.Errorf("Error counting sets:%w", err)
	}

	// Query sets by product line name
	sql := "SELECT * FROM sets WHERE product_line_name=$1;"
	rows, err := tx.Query(ctx, sql, name)
	if err != nil {
		return nil, fmt.Errorf("Error querying sets by product line name %s: %w\n", name, err)
	}

	// Scan rows into set list
	sets := make([]Set, setCount)
	for i := 0; i < len(sets); i++ {
		if !rows.Next() {
			break
		}
		s := &sets[i]
		err := rows.Scan(s.Id, s.Name, s.UrlName, s.Count)
		if err != nil {
			return nil, fmt.Errorf("Error scanning set rows for product line name %s: %w\n", name, err)
		}
	}
	// Check if loop ended due to errer or end of rows
	if rows.Err() != nil {
		return nil, fmt.Errorf("Error iterating through set rows for product line name %s: %w\n", name, err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("Error commiting query read operations of sets for product line '%s': %w", name, err)
	}

	return sets, nil
}

func (r *PostgresDataStore) GetProductsBySetName(ctx context.Context, setName string) ([]Product, error) {
	var rowCount int // Holds count of products for the specified set

	// Begin a transaction with serializable isolation level
	// which guarantees a fully consistent view of database state
	// throughout the transaction, preventing concurrency anomolies.
	txOptions := pgx.TxOptions{
		IsoLevel: pgx.Serializable,
	}

	// Begin transaction with specified tranaction options.
	tx, err := r.cp.BeginTx(ctx, txOptions)
	if err != nil {
		return nil, fmt.Errorf("Error beginning DB transaction")
	}
	defer tx.Rollback(ctx)

	// Get count of rows to be returned in the query following this one
	row := tx.QueryRow(ctx, "SELECT COUNT(*) FROM products WHERE set_name=$1;", setName)
	err = row.Scan(&rowCount)

	// Get all products in set specified in setName
	sql := "SELECT * FROM products WHERE set_name=$1;"
	rows, err := tx.Query(ctx, sql, setName)
	if err != nil {
		return nil, fmt.Errorf("Error querying product rows by set name '%s': %w\n", setName, err)
	}

	// Scan rows into product list
	products := make([]Product, rowCount) // Create slice to hold products
	var i int
	for rows.Next() {
		p := &products[i]
		i++
		err := rows.Scan(
			&p.ProductId, &p.ProductName, &p.ProductUrlName, &p.ProductLineName,
			&p.ProductLineUrlName, &p.RarityName, &p.CustomAttributes,
			&p.SetName, &p.SetUrlName, &p.ProductNumber, &p.PrintEdition,
			&p.ReleaseDate, &p.ProductLineId, &p.SetId,
		)
		if err != nil {
			return nil, fmt.Errorf("Error scanning product row for set name '%s': %w\n", setName, err)
		}

	}
	// Check if loop ended due to errer or end of rows
	rowsErr := rows.Err()
	if rowsErr != nil {
		return nil, fmt.Errorf("Error iterating through product rows for set '%s': %w\n", setName, rowsErr)
	}

	// Commit transaction
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("Error commiting query read operations")
	}

	return products, nil
}

// AddProductLine adds a new product line to the database and returns the added product line with its assigned ID.
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

// AddSets adds multiple sets to the database in a single batch operation.
// Returns the list of sets with their assigned IDs after insertion.
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
		// Scan newly inserted rows into set list to retrieve assigned IDs
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

func (r *PostgresDataStore) AddSetData(ctx context.Context, set *Set, products []Product) error {
	txOptions := pgx.TxOptions{
		IsoLevel: pgx.Serializable,
	}
	tx, err := r.cp.BeginTx(ctx, txOptions)
	if err != nil {
		return fmt.Errorf("Error beginning DB transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	setSql := "INSERT INTO sets (set_name, set_url_name, card_count, release_date, product_line_id) " +
		"VALUES ($1, $2, $3, $4, $5);"

	_, execErr := tx.Exec(ctx, setSql, set.Name, set.UrlName, set.Count, set.ReleaseDate, set.ProductLineId)
	if execErr != nil {
		return fmt.Errorf("Error inserting set info for set %s in AddSetData(): %w", set.Name, execErr)
	}

	productSql := "INSERT INTO products (product_name, product_url_name, product_line_name, " +
		"product_line_url_name, rarity_name, custom_attributes, set_name, set_url_name, " +
		"product_number, print_edition, release_date, product_line_id, set_id) " +
		"VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13);"

	batch := &pgx.Batch{} // Create a new batch for batch execution

	for _, p := range products {
		batch.Queue(
			productSql,
			p.ProductName, p.ProductUrlName, p.ProductLineName,
			p.ProductLineUrlName, p.RarityName, p.CustomAttributes,
			p.SetName, p.SetUrlName, p.ProductNumber, p.PrintEdition,
			p.ReleaseDate, p.ProductLineId, p.SetId,
		)
	}

	br := tx.SendBatch(ctx, batch)
	defer br.Close()

	for i := 0; i < batch.Len(); i++ {
		_, brErr := br.Exec()
		if brErr != nil {
			return fmt.Errorf("Error inserting products for set %s in AddSetData(): %w", set.Name, brErr)
		}
	}

	if err := br.Close(); err != nil {
		return fmt.Errorf("Error closing batch results in AddSetData(): %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("Error committing DB transaction in AddSetData(): %w", err)
	}

	return nil
}
