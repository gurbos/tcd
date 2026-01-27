CREATE TABLE  products (
    product_id INT GENERATED ALWAYS AS IDENTITY,
    product_name VARCHAR(100) NOT NULL,
    product_url_name VARCHAR(100) NOT NULL,
    product_line_name VARCHAR(100) NOT NULL,
    product_line_url_name VARCHAR(100) NOT NULL,
    rarity_name VARCHAR(50) NOT NULL,
    custom_attributes TEXT NOT NULL,
    set_name VARCHAR(100) NOT NULL,
    set_url_name VARCHAR(100) NOT NULL,
    product_number VARCHAR(30) NOT NULL,
    print_edition VARCHAR(50) NOT NULL,
    release_date VARCHAR(20) NOT NULL,
    set_id INT NOT NULL,
    product_line_id INT NOT NULL,
    PRIMARY KEY (product_number, rarity_name, set_id),
    FOREIGN KEY (set_id) REFERENCES sets(set_id),
    FOREIGN KEY (product_line_id) REFERENCES product_lines(product_line_id)
);