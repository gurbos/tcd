CREATE TABLE product_lines (
    product_line_id SERIAL UNIQUE NOT NULL,
    product_line_name VARCHAR(100) UNIQUE NOT NULL,
    product_line_url_name VARCHAR(100) UNIQUE NOT NULL,
    PRIMARY KEY (product_line_id)
);