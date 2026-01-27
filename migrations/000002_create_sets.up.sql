REATE TABLE sets (
    set_id SERIAL UNIQUE NOT NULL,
    set_name VARCHAR(100) UNIQUE NOT NULL,
    set_url_name VARCHAR(100) UNIQUE NOT NULL,
    card_count INT NOT NULL,
    release_date VARCHAR(20),
    product_line_id INT NOT NULL,
    PRIMARY KEY (set_id),
    FOREIGN KEY (product_line_id) REFERENCES product_lines(product_line_id)
);